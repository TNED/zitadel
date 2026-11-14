package wechat

import "github.com/zitadel/zitadel/internal/idp"

const (
	CredentialTypeMiniProgram = "wechat_mini_program"
	CredentialTypeNativeApp   = "wechat_native_app"

	KeyJsCode    = "js_code"
	KeyCode      = "code"
	KeyPhoneCode = "phone_code"
)

// MiniProgramCredentials represents credentials from a WeChat Mini Program.
type MiniProgramCredentials struct {
	JsCode    string
	PhoneCode string
}

var _ idp.DirectCredentials = (*MiniProgramCredentials)(nil)

func (c *MiniProgramCredentials) CredentialType() string {
	return CredentialTypeMiniProgram
}

func (c *MiniProgramCredentials) Payload() map[string]string {
	payload := map[string]string{
		KeyJsCode: c.JsCode,
	}
	if c.PhoneCode != "" {
		payload[KeyPhoneCode] = c.PhoneCode
	}
	return payload
}

// NativeAppCredentials represents credentials from an iOS/Android native app via WeChat OpenSDK.
type NativeAppCredentials struct {
	Code string
}

var _ idp.DirectCredentials = (*NativeAppCredentials)(nil)

func (c *NativeAppCredentials) CredentialType() string {
	return CredentialTypeNativeApp
}

func (c *NativeAppCredentials) Payload() map[string]string {
	return map[string]string{
		KeyCode: c.Code,
	}
}
