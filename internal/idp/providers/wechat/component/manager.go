package component

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

var (
	ErrMissingVerifyTicket = errors.New("wechat component: verify ticket is missing (wait for WeChat server push)")
	ErrComponentTokenEmpty = errors.New("wechat component: received empty component_access_token from WeChat")
)

const (
	DefaultComponentTokenEndpoint         = "https://api.weixin.qq.com/cgi-bin/component/api_component_token"
	DefaultCreatePreAuthCodeEndpoint      = "https://api.weixin.qq.com/cgi-bin/component/api_create_preauthcode"
	DefaultQueryAuthEndpoint              = "https://api.weixin.qq.com/cgi-bin/component/api_query_auth"
	DefaultRefreshAuthorizerTokenEndpoint = "https://api.weixin.qq.com/cgi-bin/component/api_authorizer_token"
)

// AuthorizerAuthInfo contains the authorization credentials returned after a tenant scans the QR code.
type AuthorizerAuthInfo struct {
	AuthorizerAppID        string `json:"authorizer_appid"`
	AuthorizerAccessToken  string `json:"authorizer_access_token"`
	ExpiresIn              int64  `json:"expires_in"`
	AuthorizerRefreshToken string `json:"authorizer_refresh_token"`
	FuncInfo               []int  `json:"func_info,omitempty"`
}

type cachedAuthorizerToken struct {
	token     string
	expiresAt time.Time
}

// Manager coordinates Third-Party Platform credentials, caching, and token lifecycles.
type Manager struct {
	componentAppID     string
	componentAppSecret string
	httpClient         *http.Client
	sf                 singleflight.Group

	mu                   sync.RWMutex
	verifyTicket         string
	componentAccessToken string
	tokenExpiresAt       time.Time

	// Multi-tenant authorizer token in-memory cache
	authorizerTokens sync.Map // map[string]*cachedAuthorizerToken

	componentTokenEndpoint         string
	createPreAuthCodeEndpoint      string
	queryAuthEndpoint              string
	refreshAuthorizerTokenEndpoint string
}

type ManagerOpt func(*Manager)

func WithCustomEndpoints(tokenURL, preAuthURL, queryAuthURL, refreshAuthURL string) ManagerOpt {
	return func(m *Manager) {
		if tokenURL != "" {
			m.componentTokenEndpoint = tokenURL
		}
		if preAuthURL != "" {
			m.createPreAuthCodeEndpoint = preAuthURL
		}
		if queryAuthURL != "" {
			m.queryAuthEndpoint = queryAuthURL
		}
		if refreshAuthURL != "" {
			m.refreshAuthorizerTokenEndpoint = refreshAuthURL
		}
	}
}

func NewManager(appID, appSecret string, httpClient *http.Client, opts ...ManagerOpt) *Manager {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 15 * time.Second,
		}
	}
	m := &Manager{
		componentAppID:                 appID,
		componentAppSecret:             appSecret,
		httpClient:                     httpClient,
		componentTokenEndpoint:         DefaultComponentTokenEndpoint,
		createPreAuthCodeEndpoint:      DefaultCreatePreAuthCodeEndpoint,
		queryAuthEndpoint:              DefaultQueryAuthEndpoint,
		refreshAuthorizerTokenEndpoint: DefaultRefreshAuthorizerTokenEndpoint,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *Manager) ComponentAppID() string {
	return m.componentAppID
}

// SetVerifyTicket updates the verify ticket pushed periodically by WeChat (every 10 minutes).
func (m *Manager) SetVerifyTicket(ticket string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifyTicket = ticket
}

func (m *Manager) GetVerifyTicket() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.verifyTicket
}

// InvalidateComponentToken clears the cached component access token.
func (m *Manager) InvalidateComponentToken() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.componentAccessToken = ""
	m.tokenExpiresAt = time.Time{}
}

// InvalidateAuthorizerToken clears the cached authorizer access token for a tenant.
func (m *Manager) InvalidateAuthorizerToken(authorizerAppID string) {
	m.authorizerTokens.Delete(authorizerAppID)
}

// GetComponentAccessToken returns a valid component_access_token, refreshing automatically if expired.
// Concurrent refresh requests are deduplicated via singleflight and the HTTP call is executed outside
// the mutex lock to prevent lock contention and request blocking.
func (m *Manager) GetComponentAccessToken(ctx context.Context) (string, error) {
	m.mu.RLock()
	if m.componentAccessToken != "" && time.Now().Before(m.tokenExpiresAt) {
		token := m.componentAccessToken
		m.mu.RUnlock()
		return token, nil
	}
	m.mu.RUnlock()

	// Use singleflight so that multiple concurrent goroutines requesting a token refresh
	// share a single in-flight network call without holding m.mu during HTTP requests.
	v, err, _ := m.sf.Do("component_access_token", func() (any, error) {
		// Double check under lock in case another singleflight or previous call updated it
		m.mu.RLock()
		if m.componentAccessToken != "" && time.Now().Before(m.tokenExpiresAt) {
			token := m.componentAccessToken
			m.mu.RUnlock()
			return token, nil
		}
		ticket := m.verifyTicket
		m.mu.RUnlock()

		if ticket == "" {
			return "", ErrMissingVerifyTicket
		}

		// Perform network call OUTSIDE of m.mu lock
		token, expiresAt, err := m.fetchComponentAccessToken(ctx, ticket)
		if err != nil {
			return "", err
		}

		m.mu.Lock()
		m.componentAccessToken = token
		m.tokenExpiresAt = expiresAt
		m.mu.Unlock()

		return token, nil
	})

	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (m *Manager) fetchComponentAccessToken(ctx context.Context, ticket string) (string, time.Time, error) {
	reqBody := map[string]string{
		"component_appid":         m.componentAppID,
		"component_appsecret":     m.componentAppSecret,
		"component_verify_ticket": ticket,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", time.Time{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.componentTokenEndpoint, bytes.NewReader(payload))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, err
	}

	var res struct {
		ComponentAccessToken string `json:"component_access_token"`
		ExpiresIn            int64  `json:"expires_in"`
		ErrCode              int    `json:"errcode"`
		ErrMsg               string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", time.Time{}, fmt.Errorf("wechat component: decode token response failed: %w", err)
	}
	if res.ErrCode != 0 {
		return "", time.Time{}, fmt.Errorf("wechat component: get token failed [errcode=%d, errmsg=%s]", res.ErrCode, res.ErrMsg)
	}
	if res.ComponentAccessToken == "" {
		return "", time.Time{}, ErrComponentTokenEmpty
	}

	// Buffer expiry time by 5 minutes to avoid race conditions
	buffer := 5 * time.Minute
	if res.ExpiresIn <= 300 {
		buffer = 10 * time.Second
	}
	expiresAt := time.Now().Add(time.Duration(res.ExpiresIn)*time.Second - buffer)

	return res.ComponentAccessToken, expiresAt, nil
}

// CreatePreAuthCode generates a pre-auth code used to construct the QR code scanning authorization URL.
func (m *Manager) CreatePreAuthCode(ctx context.Context) (string, error) {
	token, err := m.GetComponentAccessToken(ctx)
	if err != nil {
		return "", err
	}

	code, err := m.doCreatePreAuthCode(ctx, token)
	if err != nil && isTokenInvalidErr(err) {
		// Component token expired on WeChat server, clear cache and retry once
		m.InvalidateComponentToken()
		token, err = m.GetComponentAccessToken(ctx)
		if err != nil {
			return "", err
		}
		return m.doCreatePreAuthCode(ctx, token)
	}
	return code, err
}

func (m *Manager) doCreatePreAuthCode(ctx context.Context, token string) (string, error) {
	reqURL := fmt.Sprintf("%s?component_access_token=%s", m.createPreAuthCodeEndpoint, token)
	reqBody := map[string]string{
		"component_appid": m.componentAppID,
	}
	payload, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var res struct {
		PreAuthCode string `json:"pre_auth_code"`
		ExpiresIn   int64  `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", err
	}
	if res.ErrCode != 0 {
		return "", fmt.Errorf("wechat component: create pre_auth_code failed [errcode=%d, errmsg=%s]", res.ErrCode, res.ErrMsg)
	}

	return res.PreAuthCode, nil
}

// ExchangeAuthCode exchanges an authorization_code obtained after tenant QR scanning for authorizer credentials.
func (m *Manager) ExchangeAuthCode(ctx context.Context, authCode string) (*AuthorizerAuthInfo, error) {
	token, err := m.GetComponentAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	info, err := m.doExchangeAuthCode(ctx, token, authCode)
	if err != nil && isTokenInvalidErr(err) {
		m.InvalidateComponentToken()
		token, err = m.GetComponentAccessToken(ctx)
		if err != nil {
			return nil, err
		}
		return m.doExchangeAuthCode(ctx, token, authCode)
	}
	return info, err
}

func (m *Manager) doExchangeAuthCode(ctx context.Context, token, authCode string) (*AuthorizerAuthInfo, error) {
	reqURL := fmt.Sprintf("%s?component_access_token=%s", m.queryAuthEndpoint, token)
	reqBody := map[string]string{
		"component_appid":    m.componentAppID,
		"authorization_code": authCode,
	}
	payload, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res struct {
		AuthorizationInfo struct {
			AuthorizerAppID        string `json:"authorizer_appid"`
			AuthorizerAccessToken  string `json:"authorizer_access_token"`
			ExpiresIn              int64  `json:"expires_in"`
			AuthorizerRefreshToken string `json:"authorizer_refresh_token"`
		} `json:"authorization_info"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	if res.ErrCode != 0 {
		return nil, fmt.Errorf("wechat component: exchange auth_code failed [errcode=%d, errmsg=%s]", res.ErrCode, res.ErrMsg)
	}

	return &AuthorizerAuthInfo{
		AuthorizerAppID:        res.AuthorizationInfo.AuthorizerAppID,
		AuthorizerAccessToken:  res.AuthorizationInfo.AuthorizerAccessToken,
		ExpiresIn:              res.AuthorizationInfo.ExpiresIn,
		AuthorizerRefreshToken: res.AuthorizationInfo.AuthorizerRefreshToken,
	}, nil
}

// RefreshAuthorizerToken requests a fresh authorizer_access_token from WeChat using authorizer_refresh_token.
func (m *Manager) RefreshAuthorizerToken(ctx context.Context, authorizerAppID, authorizerRefreshToken string) (*AuthorizerAuthInfo, error) {
	token, err := m.GetComponentAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	info, err := m.doRefreshAuthorizerToken(ctx, token, authorizerAppID, authorizerRefreshToken)
	if err != nil && isTokenInvalidErr(err) {
		m.InvalidateComponentToken()
		token, err = m.GetComponentAccessToken(ctx)
		if err != nil {
			return nil, err
		}
		return m.doRefreshAuthorizerToken(ctx, token, authorizerAppID, authorizerRefreshToken)
	}
	return info, err
}

func (m *Manager) doRefreshAuthorizerToken(ctx context.Context, componentToken, authorizerAppID, authorizerRefreshToken string) (*AuthorizerAuthInfo, error) {
	reqURL := fmt.Sprintf("%s?component_access_token=%s", m.refreshAuthorizerTokenEndpoint, componentToken)
	reqBody := map[string]string{
		"component_appid":          m.componentAppID,
		"authorizer_appid":         authorizerAppID,
		"authorizer_refresh_token": authorizerRefreshToken,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res struct {
		AuthorizerAccessToken  string `json:"authorizer_access_token"`
		ExpiresIn              int64  `json:"expires_in"`
		AuthorizerRefreshToken string `json:"authorizer_refresh_token"`
		ErrCode                int    `json:"errcode"`
		ErrMsg                 string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	if res.ErrCode != 0 {
		return nil, fmt.Errorf("wechat component: refresh authorizer_token failed [errcode=%d, errmsg=%s]", res.ErrCode, res.ErrMsg)
	}

	return &AuthorizerAuthInfo{
		AuthorizerAppID:        authorizerAppID,
		AuthorizerAccessToken:  res.AuthorizerAccessToken,
		ExpiresIn:              res.ExpiresIn,
		AuthorizerRefreshToken: res.AuthorizerRefreshToken,
	}, nil
}

// GetAuthorizerAccessToken returns a valid authorizer_access_token, refreshing automatically if expired.
// Multiple concurrent calls for the same tenant are deduplicated using singleflight.
func (m *Manager) GetAuthorizerAccessToken(ctx context.Context, authorizerAppID, authorizerRefreshToken string) (string, error) {
	if val, ok := m.authorizerTokens.Load(authorizerAppID); ok {
		item := val.(*cachedAuthorizerToken)
		if time.Now().Before(item.expiresAt) {
			return item.token, nil
		}
	}

	sfKey := "authorizer_token:" + authorizerAppID
	v, err, _ := m.sf.Do(sfKey, func() (any, error) {
		// Double check under cache
		if val, ok := m.authorizerTokens.Load(authorizerAppID); ok {
			item := val.(*cachedAuthorizerToken)
			if time.Now().Before(item.expiresAt) {
				return item.token, nil
			}
		}

		info, err := m.RefreshAuthorizerToken(ctx, authorizerAppID, authorizerRefreshToken)
		if err != nil {
			return "", err
		}

		buffer := 5 * time.Minute
		if info.ExpiresIn <= 300 {
			buffer = 10 * time.Second
		}
		expiresAt := time.Now().Add(time.Duration(info.ExpiresIn)*time.Second - buffer)

		m.authorizerTokens.Store(authorizerAppID, &cachedAuthorizerToken{
			token:     info.AuthorizerAccessToken,
			expiresAt: expiresAt,
		})

		return info.AuthorizerAccessToken, nil
	})

	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func isTokenInvalidErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "40001") || strings.Contains(err.Error(), "invalid credential")
}
