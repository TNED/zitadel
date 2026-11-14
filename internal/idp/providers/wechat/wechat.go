package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/zitadel/zitadel/internal/idp"
)

type AppType int

const (
	AppTypeUnspecified AppType = iota
	AppTypeOfficialAccount
	AppTypeOpenPlatformWeb
	AppTypeMiniProgram
	AppTypeNativeApp
)

const (
	DefaultName = "WeChat"

	OfficialAccountAuthEndpoint = "https://open.weixin.qq.com/connect/oauth2/authorize"
	OpenPlatformWebAuthEndpoint = "https://open.weixin.qq.com/connect/qrconnect"
	TokenEndpoint               = "https://api.weixin.qq.com/sns/oauth2/access_token"
	UserInfoEndpoint            = "https://api.weixin.qq.com/sns/userinfo"
	Code2SessionEndpoint        = "https://api.weixin.qq.com/sns/jscode2session"

	ComponentOAuthTokenEndpoint   = "https://api.weixin.qq.com/sns/oauth2/component/access_token"
	ComponentCode2SessionEndpoint = "https://api.weixin.qq.com/sns/component/jscode2session"

	ScopeBase     = "snsapi_base"
	ScopeUserInfo = "snsapi_userinfo"
	ScopeLogin    = "snsapi_login"
)

var (
	ErrDirectAuthOnly  = errors.New("wechat: this app type only supports direct credential authentication (non-redirect)")
	ErrRedirectOnly    = errors.New("wechat: this app type only supports redirect-based authentication")
	ErrInvalidCreds    = errors.New("wechat: invalid or missing credentials in payload")
)

var (
	_ idp.Provider                 = (*Provider)(nil)
	_ idp.DirectCredentialProvider = (*Provider)(nil)
)

type Provider struct {
	name              string
	appID             string
	appSecret         string
	appType           AppType
	callbackURL       string
	scopes            []string
	requireUnionID    bool
	isLinkingAllowed  bool
	isCreationAllowed bool
	isAutoCreation    bool
	isAutoUpdate      bool
	httpClient        *http.Client

	authEndpoint         string
	tokenEndpoint        string
	userInfoEndpoint     string
	code2SessionEndpoint string

	isComponentMode               bool
	componentAppID                string
	componentTokenSource          func(ctx context.Context) (string, error)
	componentOAuthTokenEndpoint   string
	componentCode2SessionEndpoint string
}

type ProviderOpts func(p *Provider)

func WithLinkingAllowed() ProviderOpts {
	return func(p *Provider) { p.isLinkingAllowed = true }
}

func WithCreationAllowed() ProviderOpts {
	return func(p *Provider) { p.isCreationAllowed = true }
}

func WithAutoCreation() ProviderOpts {
	return func(p *Provider) { p.isAutoCreation = true }
}

func WithAutoUpdate() ProviderOpts {
	return func(p *Provider) { p.isAutoUpdate = true }
}

func WithRequireUnionID(required bool) ProviderOpts {
	return func(p *Provider) { p.requireUnionID = required }
}

// WithComponentMode configures the provider to run in WeChat Third-Party Platform (Component) mode.
// In this mode, ZITADEL acts as an authorized service provider on behalf of the tenant,
// using component_appid and a dynamically refreshed component_access_token without needing the tenant's app_secret.
func WithComponentMode(componentAppID string, tokenSource func(ctx context.Context) (string, error)) ProviderOpts {
	return func(p *Provider) {
		p.isComponentMode = true
		p.componentAppID = componentAppID
		p.componentTokenSource = tokenSource
		p.componentOAuthTokenEndpoint = ComponentOAuthTokenEndpoint
		p.componentCode2SessionEndpoint = ComponentCode2SessionEndpoint
	}
}

// WithCustomComponentEndpoints allows overriding Component API URLs (useful for unit tests and mock servers).
func WithCustomComponentEndpoints(oauthTokenURL, code2SessionURL string) ProviderOpts {
	return func(p *Provider) {
		if oauthTokenURL != "" {
			p.componentOAuthTokenEndpoint = oauthTokenURL
		}
		if code2SessionURL != "" {
			p.componentCode2SessionEndpoint = code2SessionURL
		}
	}
}

// WithCustomEndpoints allows overriding WeChat API URLs (useful for unit testing and mock servers).
func WithCustomEndpoints(authURL, tokenURL, userInfoURL, code2SessionURL string) ProviderOpts {
	return func(p *Provider) {
		if authURL != "" {
			p.authEndpoint = authURL
		}
		if tokenURL != "" {
			p.tokenEndpoint = tokenURL
		}
		if userInfoURL != "" {
			p.userInfoEndpoint = userInfoURL
		}
		if code2SessionURL != "" {
			p.code2SessionEndpoint = code2SessionURL
		}
	}
}

// New creates a new WeChat provider configured for the given AppType.
func New(appID, appSecret string, appType AppType, callbackURL string, scopes []string, httpClient *http.Client, options ...ProviderOpts) (*Provider, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	p := &Provider{
		name:                 DefaultName,
		appID:                appID,
		appSecret:            appSecret,
		appType:              appType,
		callbackURL:          callbackURL,
		scopes:               scopes,
		httpClient:           httpClient,
		authEndpoint:         OfficialAccountAuthEndpoint,
		tokenEndpoint:        TokenEndpoint,
		userInfoEndpoint:     UserInfoEndpoint,
		code2SessionEndpoint: Code2SessionEndpoint,
	}

	if appType == AppTypeOpenPlatformWeb {
		p.authEndpoint = OpenPlatformWebAuthEndpoint
	}

	for _, opt := range options {
		opt(p)
	}

	return p, nil
}

func (p *Provider) Name() string {
	return p.name
}

func (p *Provider) IsLinkingAllowed() bool {
	return p.isLinkingAllowed
}

func (p *Provider) IsCreationAllowed() bool {
	return p.isCreationAllowed
}

func (p *Provider) IsAutoCreation() bool {
	return p.isAutoCreation
}

func (p *Provider) IsAutoUpdate() bool {
	return p.isAutoUpdate
}

// BeginAuth initiates redirect authentication for WeChat Official Account or PC QR Connect.
func (p *Provider) BeginAuth(ctx context.Context, state string, params ...idp.Parameter) (idp.Session, error) {
	if p.appType == AppTypeMiniProgram || p.appType == AppTypeNativeApp {
		return nil, ErrDirectAuthOnly
	}

	authURL, err := p.buildAuthURL(state)
	if err != nil {
		return nil, err
	}

	return &Session{
		AuthURL:  authURL,
		State:    state,
		provider: p,
	}, nil
}

// NewSession creates an idp.Session with the callback code and state.
func (p *Provider) NewSession(code, state string) *Session {
	return &Session{
		Code:     code,
		State:    state,
		provider: p,
	}
}

func (p *Provider) buildAuthURL(state string) (string, error) {
	u, err := url.Parse(p.authEndpoint)
	if err != nil {
		return "", err
	}

	scope := ScopeUserInfo
	if len(p.scopes) > 0 {
		scope = strings.Join(p.scopes, ",")
	} else if p.appType == AppTypeOpenPlatformWeb {
		scope = ScopeLogin
	}

	q := u.Query()
	q.Set("appid", p.appID)
	q.Set("redirect_uri", p.callbackURL)
	q.Set("response_type", "code")
	q.Set("scope", scope)
	q.Set("state", state)
	if p.isComponentMode && p.componentAppID != "" {
		q.Set("component_appid", p.componentAppID)
	}
	u.RawQuery = q.Encode()

	// WeChat requires "#wechat_redirect" at the end of the authorization URL for Official Accounts
	authURL := u.String()
	if p.appType == AppTypeOfficialAccount {
		authURL += "#wechat_redirect"
	}
	return authURL, nil
}

// Code2SessionResponse models the response from WeChat code2Session API.
type Code2SessionResponse struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid,omitempty"`
	ErrCode    int    `json:"errcode,omitempty"`
	ErrMsg     string `json:"errmsg,omitempty"`
}

// VerifyDirectCredential directly exchanges temporary client credentials (Mini Program code, Native App code)
// without requiring browser redirection.
func (p *Provider) VerifyDirectCredential(ctx context.Context, creds idp.DirectCredentials) (idp.User, error) {
	if creds == nil {
		return nil, ErrInvalidCreds
	}

	switch creds.CredentialType() {
	case CredentialTypeMiniProgram:
		return p.verifyMiniProgram(ctx, creds)
	case CredentialTypeNativeApp:
		return p.verifyNativeApp(ctx, creds)
	default:
		// Fallback check payload keys
		payload := creds.Payload()
		if payload[KeyJsCode] != "" {
			return p.verifyMiniProgram(ctx, creds)
		}
		if payload[KeyCode] != "" {
			return p.verifyNativeApp(ctx, creds)
		}
		return nil, ErrInvalidCreds
	}
}

func (p *Provider) verifyMiniProgram(ctx context.Context, creds idp.DirectCredentials) (idp.User, error) {
	payload := creds.Payload()
	jsCode := payload[KeyJsCode]
	if jsCode == "" {
		jsCode = payload[KeyCode]
	}
	if jsCode == "" {
		return nil, ErrInvalidCreds
	}

	endpoint := p.code2SessionEndpoint
	if p.isComponentMode {
		endpoint = p.componentCode2SessionEndpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("appid", p.appID)
	q.Set("js_code", jsCode)
	q.Set("grant_type", "authorization_code")

	if p.isComponentMode {
		if p.componentTokenSource == nil {
			return nil, errors.New("wechat: component token source is not configured")
		}
		compToken, err := p.componentTokenSource(ctx)
		if err != nil {
			return nil, fmt.Errorf("wechat: get component_access_token failed: %w", err)
		}
		q.Set("component_appid", p.componentAppID)
		q.Set("component_access_token", compToken)
	} else {
		q.Set("secret", p.appSecret)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var sessionResp Code2SessionResponse
	if err := json.Unmarshal(body, &sessionResp); err != nil {
		return nil, fmt.Errorf("wechat: failed to decode code2Session response: %w", err)
	}

	if sessionResp.ErrCode != 0 {
		return nil, fmt.Errorf("wechat: code2Session failed [errcode=%d, errmsg=%s]", sessionResp.ErrCode, sessionResp.ErrMsg)
	}

	if p.requireUnionID && sessionResp.UnionID == "" {
		return nil, ErrUnionIDRequired
	}

	user := &User{
		OpenID:  sessionResp.OpenID,
		UnionID: sessionResp.UnionID,
	}

	return user, nil
}

func (p *Provider) verifyNativeApp(ctx context.Context, creds idp.DirectCredentials) (idp.User, error) {
	payload := creds.Payload()
	code := payload[KeyCode]
	if code == "" {
		return nil, ErrInvalidCreds
	}

	session := p.NewSession(code, "")
	return session.FetchUser(ctx)
}
