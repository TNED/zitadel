package component

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_GetComponentAccessToken_SuccessAndCache(t *testing.T) {
	var httpCallCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "component_token") {
			atomic.AddInt32(&httpCallCount, 1)
			var req map[string]string
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "my_component_appid", req["component_appid"])
			assert.Equal(t, "my_ticket_123", req["component_verify_ticket"])

			_ = json.NewEncoder(w).Encode(map[string]any{
				"component_access_token": "mock_component_token_xyz",
				"expires_in":             7200,
				"errcode":                0,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	mgr := NewManager(
		"my_component_appid",
		"my_component_secret",
		server.Client(),
		WithCustomEndpoints(server.URL+"/component_token", "", "", ""),
	)

	// Attempt before ticket arrives
	_, err := mgr.GetComponentAccessToken(context.Background())
	assert.ErrorIs(t, err, ErrMissingVerifyTicket)

	// Set ticket
	mgr.SetVerifyTicket("my_ticket_123")
	assert.Equal(t, "my_ticket_123", mgr.GetVerifyTicket())

	// First call -> calls remote API
	token1, err := mgr.GetComponentAccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "mock_component_token_xyz", token1)
	assert.Equal(t, int32(1), atomic.LoadInt32(&httpCallCount))

	// Second call -> cached, should not call remote API again
	token2, err := mgr.GetComponentAccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "mock_component_token_xyz", token2)
	assert.Equal(t, int32(1), atomic.LoadInt32(&httpCallCount), "Should return cached token without new HTTP call")
}

func TestManager_CreatePreAuthCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "component_token") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"component_access_token": "token_123",
				"expires_in":             7200,
			})
			return
		}
		if strings.Contains(r.URL.Path, "create_preauthcode") {
			assert.Equal(t, "token_123", r.URL.Query().Get("component_access_token"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"pre_auth_code": "pre_auth_code_999",
				"expires_in":    600,
				"errcode":       0,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	mgr := NewManager(
		"app_id",
		"secret",
		server.Client(),
		WithCustomEndpoints(server.URL+"/component_token", server.URL+"/create_preauthcode", "", ""),
	)
	mgr.SetVerifyTicket("ticket")

	preAuthCode, err := mgr.CreatePreAuthCode(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "pre_auth_code_999", preAuthCode)
}

func TestManager_ExchangeAuthCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "component_token") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"component_access_token": "token_123",
				"expires_in":             7200,
			})
			return
		}
		if strings.Contains(r.URL.Path, "query_auth") {
			assert.Equal(t, "token_123", r.URL.Query().Get("component_access_token"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"authorization_info": map[string]any{
					"authorizer_appid":         "tenant_appid_888",
					"authorizer_access_token":  "tenant_token_abc",
					"expires_in":               7200,
					"authorizer_refresh_token": "tenant_refresh_token_xyz",
				},
				"errcode": 0,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	mgr := NewManager(
		"app_id",
		"secret",
		server.Client(),
		WithCustomEndpoints(server.URL+"/component_token", "", server.URL+"/query_auth", ""),
	)
	mgr.SetVerifyTicket("ticket")

	authInfo, err := mgr.ExchangeAuthCode(context.Background(), "auth_code_from_callback")
	require.NoError(t, err)
	assert.Equal(t, "tenant_appid_888", authInfo.AuthorizerAppID)
	assert.Equal(t, "tenant_token_abc", authInfo.AuthorizerAccessToken)
	assert.Equal(t, "tenant_refresh_token_xyz", authInfo.AuthorizerRefreshToken)
}
