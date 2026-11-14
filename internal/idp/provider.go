package idp

import (
	"context"

	"golang.org/x/text/language"

	"github.com/zitadel/zitadel/internal/domain"
)

// Provider is the minimal implementation for a 3rd party authentication provider
type Provider interface {
	Name() string
	BeginAuth(ctx context.Context, state string, params ...Parameter) (Session, error)
	IsLinkingAllowed() bool
	IsCreationAllowed() bool
	IsAutoCreation() bool
	IsAutoUpdate() bool
}

// User contains the information of a federated user.
type User interface {
	GetID() string
	GetFirstName() string
	GetLastName() string
	GetDisplayName() string
	GetNickname() string
	GetPreferredUsername() string
	GetEmail() domain.EmailAddress
	IsEmailVerified() bool
	GetPhone() domain.PhoneNumber
	IsPhoneVerified() bool
	GetPreferredLanguage() language.Tag
	GetAvatarURL() string
	GetProfile() string
}

// Parameter allows to pass specific parameter to the BeginAuth function
type Parameter interface {
	setValue()
}

// UserAgentID allows to pass the user agent ID of the auth request to BeginAuth
type UserAgentID string

func (p UserAgentID) setValue() {}

// LoginHintParam allows to pass a login_hint to BeginAuth
type LoginHintParam string

func (p LoginHintParam) setValue() {}

// DirectCredentialProvider is an optional interface that a 3rd party authentication Provider
// can implement to support direct, non-redirect credential authentication
// (such as WeChat Mini Program code2Session, Native App SDK code exchange).
type DirectCredentialProvider interface {
	Provider
	VerifyDirectCredential(ctx context.Context, creds DirectCredentials) (User, error)
}

// DirectCredentials defines the interface for passing direct credentials (code, tokens, parameters)
// to a DirectCredentialProvider without relying on browser redirects.
type DirectCredentials interface {
	CredentialType() string
	Payload() map[string]string
}

