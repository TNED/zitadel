package domain

import (
	"context"
	"net/url"
)

type LoginPaths interface {
	DefaultEmailCodeURLTemplate(ctx context.Context) string
	DefaultPasswordSetURLTemplate(ctx context.Context) string
	DefaultPasskeySetURLTemplate(ctx context.Context) string
	DefaultDomainClaimedURLTemplate(ctx context.Context) string
}

type DefaultPaths struct {
	BasePath          *url.URL
	PasswordSetPath   *url.URL
	EmailCodePath     *url.URL
	OTPEmailPath      *url.URL
	PasskeySetPath    *url.URL
	DomainClaimedPath *url.URL
}

func (d *DefaultPaths) DefaultEmailCodeURLTemplate(ctx context.Context) string {
	if d == nil || d.EmailCodePath == nil {
		return ""
	}
	return d.EmailCodePath.String()
}

func (d *DefaultPaths) DefaultOTPEmailURLTemplate(ctx context.Context) string {
	if d == nil || d.OTPEmailPath == nil {
		return ""
	}
	return d.OTPEmailPath.String()
}

func (d *DefaultPaths) DefaultPasswordSetURLTemplate(ctx context.Context) string {
	if d == nil || d.PasswordSetPath == nil {
		return ""
	}
	return d.PasswordSetPath.String()
}

func (d *DefaultPaths) DefaultPasskeySetURLTemplate(ctx context.Context) string {
	if d == nil || d.PasskeySetPath == nil {
		return ""
	}
	return d.PasskeySetPath.String()
}

func (d *DefaultPaths) DefaultDomainClaimedURLTemplate(ctx context.Context) string {
	if d == nil || d.DomainClaimedPath == nil {
		return ""
	}
	return d.DomainClaimedPath.String()
}
