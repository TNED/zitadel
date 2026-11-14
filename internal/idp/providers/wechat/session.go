package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/zitadel/zitadel/internal/idp"
)

var (
	ErrCodeMissing     = errors.New("wechat: authorization code is missing")
	ErrUnionIDRequired = errors.New("wechat: unionid is required for cross-application linking, but not returned by WeChat")
)

var _ idp.Session = (*Session)(nil)

// Session is the [idp.Session] implementation for WeChat redirect authentication.
type Session struct {
	AuthURL   string
	Code      string
	State     string
	Expires   time.Time
	provider  *Provider
	tokens    *TokenResponse
	userInfo  *User
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"openid"`
	Scope        string `json:"scope"`
	UnionID      string `json:"unionid,omitempty"`
	ErrCode      int    `json:"errcode,omitempty"`
	ErrMsg       string `json:"errmsg,omitempty"`
}

func (s *Session) GetAuth(ctx context.Context) (idp.Auth, error) {
	return idp.Redirect(s.AuthURL)
}

func (s *Session) PersistentParameters() map[string]any {
	if s.State == "" {
		return nil
	}
	return map[string]any{"state": s.State}
}

func (s *Session) FetchUser(ctx context.Context) (idp.User, error) {
	if s.Code == "" {
		return nil, ErrCodeMissing
	}

	if s.tokens == nil {
		tokenResp, err := s.exchangeCode(ctx)
		if err != nil {
			return nil, err
		}
		s.tokens = tokenResp
		s.Expires = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	if s.provider.requireUnionID && s.tokens.UnionID == "" {
		return nil, ErrUnionIDRequired
	}

	u := &User{
		OpenID:  s.tokens.OpenID,
		UnionID: s.tokens.UnionID,
	}

	// If snsapi_userinfo scope was requested or available, fetch detailed profile
	if s.tokens.AccessToken != "" && (s.tokens.Scope == "" || s.tokens.Scope == ScopeUserInfo) {
		userInfo, err := s.fetchUserInfo(ctx, s.tokens.AccessToken, s.tokens.OpenID)
		if err == nil && userInfo != nil {
			u.Nickname = userInfo.Nickname
			u.Sex = userInfo.Sex
			u.Province = userInfo.Province
			u.City = userInfo.City
			u.Country = userInfo.Country
			u.HeadImgURL = userInfo.HeadImgURL
			u.Privilege = userInfo.Privilege
			if u.UnionID == "" && userInfo.UnionID != "" {
				u.UnionID = userInfo.UnionID
			}
		}
	}

	s.userInfo = u
	return u, nil
}

func (s *Session) ExpiresAt() time.Time {
	return s.Expires
}

func (s *Session) exchangeCode(ctx context.Context) (*TokenResponse, error) {
	endpoint := s.provider.tokenEndpoint
	if s.provider.isComponentMode {
		endpoint = s.provider.componentOAuthTokenEndpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("appid", s.provider.appID)
	q.Set("code", s.Code)
	q.Set("grant_type", "authorization_code")

	if s.provider.isComponentMode {
		if s.provider.componentTokenSource == nil {
			return nil, errors.New("wechat: component token source is not configured")
		}
		compToken, err := s.provider.componentTokenSource(ctx)
		if err != nil {
			return nil, fmt.Errorf("wechat: get component_access_token failed: %w", err)
		}
		q.Set("component_appid", s.provider.componentAppID)
		q.Set("component_access_token", compToken)
	} else {
		q.Set("secret", s.provider.appSecret)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.provider.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("wechat: failed to decode token response: %w", err)
	}

	if tokenResp.ErrCode != 0 {
		return nil, fmt.Errorf("wechat: token exchange failed [errcode=%d, errmsg=%s]", tokenResp.ErrCode, tokenResp.ErrMsg)
	}

	return &tokenResp, nil
}

func (s *Session) fetchUserInfo(ctx context.Context, accessToken, openID string) (*User, error) {
	u, err := url.Parse(s.provider.userInfoEndpoint)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("access_token", accessToken)
	q.Set("openid", openID)
	q.Set("lang", "zh_CN")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.provider.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var errCheck struct {
		ErrCode int    `json:"errcode,omitempty"`
		ErrMsg  string `json:"errmsg,omitempty"`
	}
	if err := json.Unmarshal(body, &errCheck); err == nil && errCheck.ErrCode != 0 {
		return nil, fmt.Errorf("wechat: fetch userinfo failed [errcode=%d, errmsg=%s]", errCheck.ErrCode, errCheck.ErrMsg)
	}

	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}
	return &user, nil
}
