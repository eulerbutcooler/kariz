package auth

import "errors"

type Authenticator interface {
	Authenticate(secret string) (Identity, error)
}

type Identity struct {
}

var ErrAuthFailed = errors.New("authentication failed")
var ErrTokenExpired = errors.New("token expired")
