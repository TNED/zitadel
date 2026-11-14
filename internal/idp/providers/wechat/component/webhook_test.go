package component

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookHandler_EncryptAndDecrypt_RoundTrip(t *testing.T) {
	componentAppID := "wx1122334455667788"
	token := "platform_token_123"
	// 43-character EncodingAESKey
	encodingAESKey := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

	handler, err := NewWebhookHandler(componentAppID, token, encodingAESKey)
	require.NoError(t, err)

	originalXML := `<xml><AppId>` + componentAppID + `</AppId><CreateTime>1700000000</CreateTime><InfoType>component_verify_ticket</InfoType><ComponentVerifyTicket>ticket@@@abcdef123456</ComponentVerifyTicket></xml>`

	encryptedBase64, err := handler.EncryptMessage(nil, []byte(originalXML))
	require.NoError(t, err)

	timestamp := "1700000000"
	nonce := "random_nonce_999"

	// Calculate signature
	strs := []string{token, timestamp, nonce, encryptedBase64}
	sort.Strings(strs)
	hSha := sha1.New()
	_, _ = hSha.Write([]byte(strings.Join(strs, "")))
	signature := hex.EncodeToString(hSha.Sum(nil))

	rawPayload := EncryptedXMLPayload{
		AppID:   componentAppID,
		Encrypt: encryptedBase64,
	}
	rawXMLBytes, err := xml.Marshal(rawPayload)
	require.NoError(t, err)

	event, err := handler.HandleEvent(signature, timestamp, nonce, rawXMLBytes)
	require.NoError(t, err)

	assert.Equal(t, componentAppID, event.AppID)
	assert.Equal(t, "component_verify_ticket", event.InfoType)
	assert.Equal(t, "ticket@@@abcdef123456", event.ComponentVerifyTicket)
}

func TestWebhookHandler_SignatureMismatch(t *testing.T) {
	handler, err := NewWebhookHandler("appid", "token", "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG")
	require.NoError(t, err)

	rawPayload := EncryptedXMLPayload{
		AppID:   "appid",
		Encrypt: "some_encrypt_data",
	}
	rawXMLBytes, _ := xml.Marshal(rawPayload)

	_, err = handler.HandleEvent("wrong_signature", "123", "456", rawXMLBytes)
	assert.ErrorIs(t, err, ErrSignatureMismatch)
}

func TestWebhookHandler_UnauthorizedEvent(t *testing.T) {
	componentAppID := "wx_component_999"
	token := "token999"
	encodingAESKey := "1234567890123456789012345678901234567890123"

	handler, err := NewWebhookHandler(componentAppID, token, encodingAESKey)
	require.NoError(t, err)

	unauthXML := `<xml><AppId>` + componentAppID + `</AppId><CreateTime>1700000100</CreateTime><InfoType>unauthorized</InfoType><AuthorizerAppid>tenant_wx_appid_888</AuthorizerAppid></xml>`

	encryptedBase64, err := handler.EncryptMessage(nil, []byte(unauthXML))
	require.NoError(t, err)

	timestamp := "1700000100"
	nonce := "nonce100"

	strs := []string{token, timestamp, nonce, encryptedBase64}
	sort.Strings(strs)
	hSha := sha1.New()
	_, _ = hSha.Write([]byte(strings.Join(strs, "")))
	signature := hex.EncodeToString(hSha.Sum(nil))

	rawPayload := EncryptedXMLPayload{
		AppID:   componentAppID,
		Encrypt: encryptedBase64,
	}
	rawBytes, _ := xml.Marshal(rawPayload)

	event, err := handler.HandleEvent(signature, timestamp, nonce, rawBytes)
	require.NoError(t, err)
	assert.Equal(t, "unauthorized", event.InfoType)
	assert.Equal(t, "tenant_wx_appid_888", event.AuthorizerAppID)
}
