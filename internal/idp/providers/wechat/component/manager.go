package component

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
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
	AuthorizerAppID        string   `json:"authorizer_appid"`
	AuthorizerAccessToken  string   `json:"authorizer_access_token"`
	ExpiresIn              int64    `json:"expires_in"`
	AuthorizerRefreshToken string   `json:"authorizer_refresh_token"`
	FuncInfo               []int    `json:"func_info,omitempty"`
}

// Manager coordinates Third-Party Platform credentials, caching, and token lifecycles.
type Manager struct {
	componentAppID     string
	componentAppSecret string
	httpClient         *http.Client

	mu                   sync.RWMutex
	verifyTicket         string
	componentAccessToken string
	tokenExpiresAt       time.Time

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
		httpClient = http.DefaultClient
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

// GetComponentAccessToken returns a valid component_access_token, refreshing automatically if expired.
func (m *Manager) GetComponentAccessToken(ctx context.Context) (string, error) {
	m.mu.RLock()
	if m.componentAccessToken != "" && time.Now().Before(m.tokenExpiresAt) {
		token := m.componentAccessToken
		m.mu.RUnlock()
		return token, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check after obtaining write lock
	if m.componentAccessToken != "" && time.Now().Before(m.tokenExpiresAt) {
		return m.componentAccessToken, nil
	}

	if m.verifyTicket == "" {
		return "", ErrMissingVerifyTicket
	}

	reqBody := map[string]string{
		"component_appid":         m.componentAppID,
		"component_appsecret":     m.componentAppSecret,
		"component_verify_ticket": m.verifyTicket,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.componentTokenEndpoint, bytes.NewReader(payload))
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
		ComponentAccessToken string `json:"component_access_token"`
		ExpiresIn            int64  `json:"expires_in"`
		ErrCode              int    `json:"errcode"`
		ErrMsg               string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("wechat component: decode token response failed: %w", err)
	}
	if res.ErrCode != 0 {
		return "", fmt.Errorf("wechat component: get token failed [errcode=%d, errmsg=%s]", res.ErrCode, res.ErrMsg)
	}
	if res.ComponentAccessToken == "" {
		return "", ErrComponentTokenEmpty
	}

	m.componentAccessToken = res.ComponentAccessToken
	// Buffer expiry time by 5 minutes to avoid race conditions
	buffer := 5 * time.Minute
	if res.ExpiresIn <= 300 {
		buffer = 10 * time.Second
	}
	m.tokenExpiresAt = time.Now().Add(time.Duration(res.ExpiresIn)*time.Second - buffer)

	return m.componentAccessToken, nil
}

// CreatePreAuthCode generates a pre-auth code used to construct the QR code scanning authorization URL.
func (m *Manager) CreatePreAuthCode(ctx context.Context) (string, error) {
	token, err := m.GetComponentAccessToken(ctx)
	if err != nil {
		return "", err
	}

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
