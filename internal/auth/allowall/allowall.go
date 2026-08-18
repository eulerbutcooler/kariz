package allowall

import "github.com/eulerbutcooler/kariz/internal/auth"

type AllowAll struct{}

func NewAllowAll() *AllowAll {
	return &AllowAll{}
}

func (a *AllowAll) Authenticate(secret string) (auth.Identity, error) {
	return auth.Identity{}, nil
}
