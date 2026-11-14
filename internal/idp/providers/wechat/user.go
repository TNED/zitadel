package wechat

import (
	"golang.org/x/text/language"

	"github.com/zitadel/zitadel/internal/domain"
	"github.com/zitadel/zitadel/internal/idp"
)

var _ idp.User = (*User)(nil)

// User represents a WeChat authenticated user.
type User struct {
	OpenID     string   `json:"openid"`
	UnionID    string   `json:"unionid,omitempty"`
	Nickname   string   `json:"nickname"`
	Sex        int      `json:"sex"`
	Province   string   `json:"province"`
	City       string   `json:"city"`
	Country    string   `json:"country"`
	HeadImgURL string   `json:"headimgurl"`
	Privilege  []string `json:"privilege"`

	PhoneNumber   string `json:"phone_number,omitempty"`
	PhoneVerified bool   `json:"phone_verified,omitempty"`
}

// GetID returns the federated user identifier.
// When UnionID is present, it is preferred to ensure cross-application identity matching
// across Mini Programs, Official Accounts, and Mobile Apps under the same WeChat Open Platform account.
// Otherwise, it falls back to OpenID.
func (u *User) GetID() string {
	if u.UnionID != "" {
		return u.UnionID
	}
	return u.OpenID
}

func (u *User) GetFirstName() string {
	return ""
}

func (u *User) GetLastName() string {
	return ""
}

func (u *User) GetDisplayName() string {
	if u.Nickname != "" {
		return u.Nickname
	}
	return u.GetID()
}

func (u *User) GetNickname() string {
	return u.Nickname
}

func (u *User) GetPreferredUsername() string {
	return u.GetDisplayName()
}

func (u *User) GetEmail() domain.EmailAddress {
	return ""
}

func (u *User) IsEmailVerified() bool {
	return false
}

func (u *User) GetPhone() domain.PhoneNumber {
	return domain.PhoneNumber(u.PhoneNumber)
}

func (u *User) IsPhoneVerified() bool {
	return u.PhoneVerified
}

func (u *User) GetPreferredLanguage() language.Tag {
	return language.SimplifiedChinese
}

func (u *User) GetAvatarURL() string {
	return u.HeadImgURL
}

func (u *User) GetProfile() string {
	return ""
}
