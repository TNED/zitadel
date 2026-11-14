package wechat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zitadel/zitadel/internal/idp"
)

func TestProvider_BeginAuth_OfficialAccount(t *testing.T) {
	appID := "wx1234567890abcdef"
	appSecret := "secret1234567890"
	callbackURL := "https://auth.example.com/idp/callback"

	provider, err := New(appID, appSecret, AppTypeOfficialAccount, callbackURL, []string{ScopeUserInfo}, http.DefaultClient)
	require.NoError(t, err)

	session, err := provider.BeginAuth(context.Background(), "test-state-123")
	require.NoError(t, err)

	auth, err := session.GetAuth(context.Background())
	require.NoError(t, err)

	redirectAuth, ok := auth.(*idp.RedirectAuth)
	require.True(t, ok)

	assert.Contains(t, redirectAuth.RedirectURL, OfficialAccountAuthEndpoint)
	assert.Contains(t, redirectAuth.RedirectURL, "appid="+appID)
	assert.Contains(t, redirectAuth.RedirectURL, "state=test-state-123")
	assert.Contains(t, redirectAuth.RedirectURL, "scope=snsapi_userinfo")
	assert.True(t, strings.HasSuffix(redirectAuth.RedirectURL, "#wechat_redirect"), "Official Account URL must end with #wechat_redirect")
}

func TestProvider_BeginAuth_DirectTypes_Error(t *testing.T) {
	mpProvider, err := New("app_id", "secret", AppTypeMiniProgram, "", nil, http.DefaultClient)
	require.NoError(t, err)

	_, err = mpProvider.BeginAuth(context.Background(), "state")
	assert.ErrorIs(t, err, ErrDirectAuthOnly)

	appProvider, err := New("app_id", "secret", AppTypeNativeApp, "", nil, http.DefaultClient)
	require.NoError(t, err)

	_, err = appProvider.BeginAuth(context.Background(), "state")
	assert.ErrorIs(t, err, ErrDirectAuthOnly)
}

func TestProvider_RedirectSession_FetchUser_Success(t *testing.T) {
	expectedUnionID := "unionid_wechat_global_123"
	expectedOpenID := "openid_oa_456"
	expectedNickname := "WeChatDeveloper"
	expectedAvatar := "https://thirdwx.qlogo.cn/avatar.png"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "access_token") {
			assert.Equal(t, "code_from_callback", r.URL.Query().Get("code"))
			assert.Equal(t, "appid_test", r.URL.Query().Get("appid"))
			_ = json.NewEncoder(w).Encode(TokenResponse{
				AccessToken: "mock_access_token",
				ExpiresIn:   7200,
				OpenID:      expectedOpenID,
				UnionID:     expectedUnionID,
				Scope:       ScopeUserInfo,
			})
			return
		}
		if strings.Contains(r.URL.Path, "userinfo") {
			assert.Equal(t, "mock_access_token", r.URL.Query().Get("access_token"))
			assert.Equal(t, expectedOpenID, r.URL.Query().Get("openid"))
			_ = json.NewEncoder(w).Encode(User{
				OpenID:     expectedOpenID,
				UnionID:    expectedUnionID,
				Nickname:   expectedNickname,
				HeadImgURL: expectedAvatar,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider, err := New(
		"appid_test",
		"secret_test",
		AppTypeOfficialAccount,
		"https://example.com/callback",
		[]string{ScopeUserInfo},
		server.Client(),
		WithCustomEndpoints(server.URL+"/auth", server.URL+"/access_token", server.URL+"/userinfo", server.URL+"/code2session"),
		WithRequireUnionID(true),
	)
	require.NoError(t, err)

	session := provider.NewSession("code_from_callback", "state123")
	user, err := session.FetchUser(context.Background())
	require.NoError(t, err)

	assert.Equal(t, expectedUnionID, user.GetID(), "User ID should be UnionID when present")
	assert.Equal(t, expectedNickname, user.GetDisplayName())
	assert.Equal(t, expectedAvatar, user.GetAvatarURL())
}

func TestProvider_RedirectSession_RequireUnionID_Missing_Fails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TokenResponse{
			AccessToken: "mock_access_token",
			ExpiresIn:   7200,
			OpenID:      "only_openid",
			UnionID:     "", // missing UnionID
		})
	}))
	defer server.Close()

	provider, err := New(
		"appid_test",
		"secret_test",
		AppTypeOfficialAccount,
		"https://example.com/callback",
		nil,
		server.Client(),
		WithCustomEndpoints(server.URL+"/auth", server.URL+"/access_token", server.URL+"/userinfo", server.URL+"/code2session"),
		WithRequireUnionID(true),
	)
	require.NoError(t, err)

	session := provider.NewSession("test_code", "state123")
	_, err = session.FetchUser(context.Background())
	assert.ErrorIs(t, err, ErrUnionIDRequired)
}

func TestProvider_VerifyDirectCredential_MiniProgram_Success(t *testing.T) {
	expectedUnionID := "union_mp_999"
	expectedOpenID := "openid_mp_888"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.Equal(t, "test_js_code", r.URL.Query().Get("js_code"))
		assert.Equal(t, "appid_mp", r.URL.Query().Get("appid"))
		assert.Equal(t, "authorization_code", r.URL.Query().Get("grant_type"))

		_ = json.NewEncoder(w).Encode(Code2SessionResponse{
			OpenID:     expectedOpenID,
			SessionKey: "session_key_secret",
			UnionID:    expectedUnionID,
			ErrCode:    0,
			ErrMsg:     "ok",
		})
	}))
	defer server.Close()

	provider, err := New(
		"appid_mp",
		"secret_mp",
		AppTypeMiniProgram,
		"",
		nil,
		server.Client(),
		WithCustomEndpoints("", "", "", server.URL+"/code2session"),
	)
	require.NoError(t, err)

	creds := &MiniProgramCredentials{
		JsCode: "test_js_code",
	}

	user, err := provider.VerifyDirectCredential(context.Background(), creds)
	require.NoError(t, err)
	assert.Equal(t, expectedUnionID, user.GetID())

	// Fallback when UnionID is empty
	wechatUser := user.(*User)
	assert.Equal(t, expectedOpenID, wechatUser.OpenID)
}

func TestProvider_VerifyDirectCredential_MiniProgram_WeChatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Code2SessionResponse{
			ErrCode: 40029,
			ErrMsg:  "invalid code",
		})
	}))
	defer server.Close()

	provider, err := New(
		"appid_mp",
		"secret_mp",
		AppTypeMiniProgram,
		"",
		nil,
		server.Client(),
		WithCustomEndpoints("", "", "", server.URL+"/code2session"),
	)
	require.NoError(t, err)

	creds := &MiniProgramCredentials{
		JsCode: "invalid_code",
	}

	_, err = provider.VerifyDirectCredential(context.Background(), creds)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "40029")
	assert.Contains(t, err.Error(), "invalid code")
}

func TestProvider_ComponentMode_OfficialAccount_AuthURL(t *testing.T) {
	appID := "tenant_wx_appid_111"
	componentAppID := "platform_component_appid_999"

	provider, err := New(
		appID,
		"", // In Component mode, tenant app_secret is empty
		AppTypeOfficialAccount,
		"https://auth.example.com/idp/callback",
		[]string{ScopeUserInfo},
		http.DefaultClient,
		WithComponentMode(componentAppID, func(ctx context.Context) (string, error) {
			return "mock_component_token", nil
		}),
	)
	require.NoError(t, err)

	session, err := provider.BeginAuth(context.Background(), "state_comp_123")
	require.NoError(t, err)

	auth, err := session.GetAuth(context.Background())
	require.NoError(t, err)

	redirectAuth, ok := auth.(*idp.RedirectAuth)
	require.True(t, ok)

	assert.Contains(t, redirectAuth.RedirectURL, "appid="+appID)
	assert.Contains(t, redirectAuth.RedirectURL, "component_appid="+componentAppID)
	assert.True(t, strings.HasSuffix(redirectAuth.RedirectURL, "#wechat_redirect"))
}

func TestProvider_ComponentMode_Session_FetchUser_Success(t *testing.T) {
	tenantAppID := "tenant_appid_888"
	componentAppID := "platform_comp_appid_777"
	expectedUnionID := "unionid_component_user_001"
	expectedOpenID := "openid_component_user_002"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "component_oauth_token") {
			assert.Equal(t, tenantAppID, r.URL.Query().Get("appid"))
			assert.Equal(t, componentAppID, r.URL.Query().Get("component_appid"))
			assert.Equal(t, "comp_access_token_token", r.URL.Query().Get("component_access_token"))
			assert.Empty(t, r.URL.Query().Get("secret"), "Tenant secret must not be passed in component mode")
			assert.Equal(t, "code_from_callback", r.URL.Query().Get("code"))

			_ = json.NewEncoder(w).Encode(TokenResponse{
				AccessToken: "token_for_user",
				ExpiresIn:   7200,
				OpenID:      expectedOpenID,
				UnionID:     expectedUnionID,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider, err := New(
		tenantAppID,
		"",
		AppTypeOfficialAccount,
		"https://example.com/callback",
		nil,
		server.Client(),
		WithComponentMode(componentAppID, func(ctx context.Context) (string, error) {
			return "comp_access_token_token", nil
		}),
		WithCustomEndpoints("", "", "", ""),
		WithCustomComponentEndpoints(server.URL+"/component_oauth_token", ""),
	)
	require.NoError(t, err)

	session := provider.NewSession("code_from_callback", "state123")
	user, err := session.FetchUser(context.Background())
	require.NoError(t, err)

	assert.Equal(t, expectedUnionID, user.GetID())
}

func TestProvider_ComponentMode_VerifyDirectCredential_MiniProgram(t *testing.T) {
	tenantAppID := "tenant_mp_appid_666"
	componentAppID := "platform_comp_appid_777"
	expectedUnionID := "unionid_mp_component_003"
	expectedOpenID := "openid_mp_component_004"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "component_jscode2session") {
			assert.Equal(t, tenantAppID, r.URL.Query().Get("appid"))
			assert.Equal(t, componentAppID, r.URL.Query().Get("component_appid"))
			assert.Equal(t, "comp_token_val", r.URL.Query().Get("component_access_token"))
			assert.Empty(t, r.URL.Query().Get("secret"), "Tenant secret must not be passed in component mode")
			assert.Equal(t, "mp_js_code_123", r.URL.Query().Get("js_code"))

			_ = json.NewEncoder(w).Encode(Code2SessionResponse{
				OpenID:     expectedOpenID,
				SessionKey: "sess_key",
				UnionID:    expectedUnionID,
				ErrCode:    0,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider, err := New(
		tenantAppID,
		"",
		AppTypeMiniProgram,
		"",
		nil,
		server.Client(),
		WithComponentMode(componentAppID, func(ctx context.Context) (string, error) {
			return "comp_token_val", nil
		}),
		WithCustomComponentEndpoints("", server.URL+"/component_jscode2session"),
	)
	require.NoError(t, err)

	creds := &MiniProgramCredentials{
		JsCode: "mp_js_code_123",
	}

	user, err := provider.VerifyDirectCredential(context.Background(), creds)
	require.NoError(t, err)
	assert.Equal(t, expectedUnionID, user.GetID())
}

