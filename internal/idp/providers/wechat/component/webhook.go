package component

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrSignatureMismatch = errors.New("wechat webhook: signature verification failed")
	ErrInvalidAESKey     = errors.New("wechat webhook: invalid encoding aes key")
	ErrInvalidAppID      = errors.New("wechat webhook: appid in message does not match component appid")
)

// EncryptedXMLPayload models the raw incoming XML pushed by WeChat servers.
type EncryptedXMLPayload struct {
	XMLName xml.Name `xml:"xml"`
	AppID   string   `xml:"AppId"`
	Encrypt string   `xml:"Encrypt"`
}

// PushEvent represents the decrypted message content from WeChat.
type PushEvent struct {
	XMLName                      xml.Name `xml:"xml"`
	AppID                        string   `xml:"AppId"`
	CreateTime                   int64    `xml:"CreateTime"`
	InfoType                     string   `xml:"InfoType"`
	ComponentVerifyTicket        string   `xml:"ComponentVerifyTicket,omitempty"`
	AuthorizerAppID              string   `xml:"AuthorizerAppid,omitempty"`
	AuthorizationCode            string   `xml:"AuthorizationCode,omitempty"`
	AuthorizationCodeExpiredTime int64    `xml:"AuthorizationCodeExpiredTime,omitempty"`
	PreAuthCode                  string   `xml:"PreAuthCode,omitempty"`
}

// WebhookHandler handles cryptographic verification and parsing of WeChat push messages.
type WebhookHandler struct {
	componentAppID string
	token          string
	aesKey         []byte
}

func NewWebhookHandler(componentAppID, token, encodingAESKey string) (*WebhookHandler, error) {
	if len(encodingAESKey) != 43 {
		return nil, ErrInvalidAESKey
	}
	key, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return nil, fmt.Errorf("wechat webhook: base64 decode aes key failed: %w", err)
	}
	if len(key) != 32 {
		return nil, ErrInvalidAESKey
	}

	return &WebhookHandler{
		componentAppID: componentAppID,
		token:          token,
		aesKey:         key,
	}, nil
}

// VerifySignature validates SHA1(sort(token, timestamp, nonce, encryptedMsg)) == msgSignature.
// Optimized with in-place sorting and zero heap allocations.
func (h *WebhookHandler) VerifySignature(msgSignature, timestamp, nonce, encryptedMsg string) bool {
	strs := [4]string{h.token, timestamp, nonce, encryptedMsg}
	if strs[0] > strs[1] {
		strs[0], strs[1] = strs[1], strs[0]
	}
	if strs[2] > strs[3] {
		strs[2], strs[3] = strs[3], strs[2]
	}
	if strs[0] > strs[2] {
		strs[0], strs[2] = strs[2], strs[0]
	}
	if strs[1] > strs[3] {
		strs[1], strs[3] = strs[3], strs[1]
	}
	if strs[1] > strs[2] {
		strs[1], strs[2] = strs[2], strs[1]
	}

	hSha := sha1.New()
	for _, s := range strs {
		_, _ = io.WriteString(hSha, s)
	}
	calculated := hex.EncodeToString(hSha.Sum(nil))
	return strings.EqualFold(calculated, msgSignature)
}

// DecryptMessage decrypts the base64-encoded encrypted XML payload.
func (h *WebhookHandler) DecryptMessage(encryptedBase64 string) ([]byte, error) {
	cipherText, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return nil, fmt.Errorf("wechat webhook: base64 decode cipherText failed: %w", err)
	}

	block, err := aes.NewCipher(h.aesKey)
	if err != nil {
		return nil, err
	}

	if len(cipherText) < aes.BlockSize || len(cipherText)%aes.BlockSize != 0 {
		return nil, errors.New("wechat webhook: cipherText size invalid")
	}

	iv := h.aesKey[:16]
	mode := cipher.NewCBCDecrypter(block, iv)
	plain := make([]byte, len(cipherText))
	mode.CryptBlocks(plain, cipherText)

	// PKCS#7 unpadding
	plain, err = pkcs7Unpad(plain)
	if err != nil {
		return nil, err
	}

	// WeChat format: 16-byte random + 4-byte network byte order length + msg + appid
	if len(plain) < 20 {
		return nil, errors.New("wechat webhook: decrypted message too short")
	}

	msgLen := binary.BigEndian.Uint32(plain[16:20])
	if int(20+msgLen) > len(plain) {
		return nil, errors.New("wechat webhook: invalid message length in decrypted payload")
	}

	msg := plain[20 : 20+msgLen]
	fromAppID := string(plain[20+msgLen:])
	if fromAppID != h.componentAppID {
		return nil, fmt.Errorf("%w: expected %s, got %s", ErrInvalidAppID, h.componentAppID, fromAppID)
	}

	return msg, nil
}

// HandleEvent decrypts, parses, and returns the push event.
func (h *WebhookHandler) HandleEvent(msgSignature, timestamp, nonce string, rawXML []byte) (*PushEvent, error) {
	var payload EncryptedXMLPayload
	if err := xml.Unmarshal(rawXML, &payload); err != nil {
		return nil, fmt.Errorf("wechat webhook: unmarshal raw xml failed: %w", err)
	}

	if !h.VerifySignature(msgSignature, timestamp, nonce, payload.Encrypt) {
		return nil, ErrSignatureMismatch
	}

	plainXML, err := h.DecryptMessage(payload.Encrypt)
	if err != nil {
		return nil, err
	}

	var event PushEvent
	if err := xml.Unmarshal(plainXML, &event); err != nil {
		return nil, fmt.Errorf("wechat webhook: unmarshal decrypted event failed: %w", err)
	}

	return &event, nil
}

// DispatchEvent decrypts, parses, and automatically updates the Manager's verify ticket
// if the event is a component_verify_ticket push.
func (h *WebhookHandler) DispatchEvent(msgSignature, timestamp, nonce string, rawXML []byte, mgr *Manager) (*PushEvent, error) {
	event, err := h.HandleEvent(msgSignature, timestamp, nonce, rawXML)
	if err != nil {
		return nil, err
	}

	if mgr != nil && event.InfoType == "component_verify_ticket" && event.ComponentVerifyTicket != "" {
		mgr.SetVerifyTicket(event.ComponentVerifyTicket)
	}

	return event, nil
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, errors.New("pkcs7: empty block")
	}
	padding := int(data[length-1])
	if padding > length || padding > 32 || padding == 0 {
		return nil, errors.New("pkcs7: invalid padding")
	}
	for i := length - padding; i < length; i++ {
		if data[i] != byte(padding) {
			return nil, errors.New("pkcs7: invalid padding bytes")
		}
	}
	return data[:length-padding], nil
}

// EncryptMessage helper for unit tests and outbound message encryption.
func (h *WebhookHandler) EncryptMessage(random16 []byte, msg []byte) (string, error) {
	if len(random16) != 16 {
		random16 = bytes.Repeat([]byte("a"), 16)
	}

	buf := new(bytes.Buffer)
	buf.Write(random16)
	_ = binary.Write(buf, binary.BigEndian, uint32(len(msg)))
	buf.Write(msg)
	buf.WriteString(h.componentAppID)

	// PKCS#7 pad to 32 bytes block size
	plain := pkcs7Pad(buf.Bytes(), 32)

	block, err := aes.NewCipher(h.aesKey)
	if err != nil {
		return "", err
	}

	iv := h.aesKey[:16]
	mode := cipher.NewCBCEncrypter(block, iv)
	cipherText := make([]byte, len(plain))
	mode.CryptBlocks(cipherText, plain)

	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}
