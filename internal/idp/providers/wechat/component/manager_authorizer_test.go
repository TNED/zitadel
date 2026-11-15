package component

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_RefreshAuthorizerToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "component_token") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"component_access_token": "token_comp_123",
				"expires_in":             7200,
			})
			return
		}
		if strings.Contains(r.URL.Path, "refresh_authorizer_token") {
			assert.Equal(t, "token_comp_123", r.URL.Query().Get("component_access_token"))
			var req map[string]string
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "comp_app_1", req["component_appid"])
			assert.Equal(t, "tenant_app_1", req["authorizer_appid"])
			assert.Equal(t, "ref_token_1", req["authorizer_refresh_token"])

			_ = json.NewEncoder(w).Encode(map[string]any{
				"authorizer_access_token":  "new_authorizer_token_999",
				"expires_in":               7200,
				"authorizer_refresh_token": "new_ref_token_2",
				"errcode":                  0,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	mgr := NewManager(
		"comp_app_1",
		"comp_sec_1",
		server.Client(),
		WithCustomEndpoints(server.URL+"/component_token", "", "", server.URL+"/refresh_authorizer_token"),
	)
	mgr.SetVerifyTicket("ticket_123")

	info, err := mgr.RefreshAuthorizerToken(context.Background(), "tenant_app_1", "ref_token_1")
	require.NoError(t, err)
	assert.Equal(t, "tenant_app_1", info.AuthorizerAppID)
	assert.Equal(t, "new_authorizer_token_999", info.AuthorizerAccessToken)
	assert.Equal(t, "new_ref_token_2", info.AuthorizerRefreshToken)
}

func TestManager_GetAuthorizerAccessToken_CachingAndSingleflight(t *testing.T) {
	var httpCallCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "component_token") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"component_access_token": "comp_tok",
				"expires_in":             7200,
			})
			return
		}
		if strings.Contains(r.URL.Path, "refresh_authorizer_token") {
			atomic.AddInt32(&httpCallCount, 1)
			time.Sleep(30 * time.Millisecond) // concurrency window
			_ = json.NewEncoder(w).Encode(map[string]any{
				"authorizer_access_token":  "auth_tok_concurrent",
				"expires_in":               7200,
				"authorizer_refresh_token": "new_ref_tok",
				"errcode":                  0,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	mgr := NewManager(
		"comp_app_1",
		"comp_sec_1",
		server.Client(),
		WithCustomEndpoints(server.URL+"/component_token", "", "", server.URL+"/refresh_authorizer_token"),
	)
	mgr.SetVerifyTicket("ticket_xyz")

	const goroutines = 15
	var wg sync.WaitGroup
	tokens := make(chan string, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok, err := mgr.GetAuthorizerAccessToken(context.Background(), "tenant_app_1", "ref_token_1")
			if err == nil {
				tokens <- tok
			}
		}()
	}

	wg.Wait()
	close(tokens)

	for tok := range tokens {
		assert.Equal(t, "auth_tok_concurrent", tok)
	}

	// Singleflight must ensure only 1 refresh HTTP call for the authorizer token was fired
	assert.Equal(t, int32(1), atomic.LoadInt32(&httpCallCount))

	// Subsequent sequential call uses cache
	tok, err := mgr.GetAuthorizerAccessToken(context.Background(), "tenant_app_1", "ref_token_1")
	require.NoError(t, err)
	assert.Equal(t, "auth_tok_concurrent", tok)
	assert.Equal(t, int32(1), atomic.LoadInt32(&httpCallCount))
}

func TestManager_AutoRecoveryOnTokenExpired40001(t *testing.T) {
	var compTokenRequests int32
	var refreshRequests int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "component_token") {
			count := atomic.AddInt32(&compTokenRequests, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"component_access_token": "comp_tok_v" + string('0'+rune(count)),
				"expires_in":             7200,
			})
			return
		}
		if strings.Contains(r.URL.Path, "refresh_authorizer_token") {
			count := atomic.AddInt32(&refreshRequests, 1)
			if count == 1 {
				// Simulate token expired on first try
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errcode": 40001,
					"errmsg":  "invalid credential, access_token is invalid or not latest",
				})
				return
			}
			// Success on retry with refreshed component token
			assert.Equal(t, "comp_tok_v2", r.URL.Query().Get("component_access_token"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"authorizer_access_token":  "recovered_auth_tok",
				"expires_in":               7200,
				"authorizer_refresh_token": "next_ref",
				"errcode":                  0,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	mgr := NewManager(
		"comp_app_1",
		"comp_sec_1",
		server.Client(),
		WithCustomEndpoints(server.URL+"/component_token", "", "", server.URL+"/refresh_authorizer_token"),
	)
	mgr.SetVerifyTicket("ticket_abc")

	info, err := mgr.RefreshAuthorizerToken(context.Background(), "tenant_app_1", "ref_1")
	require.NoError(t, err)
	assert.Equal(t, "recovered_auth_tok", info.AuthorizerAccessToken)
	assert.Equal(t, int32(2), atomic.LoadInt32(&compTokenRequests), "Should have refreshed component token after 40001")
	assert.Equal(t, int32(2), atomic.LoadInt32(&refreshRequests), "Should have retried the authorizer request")
}
