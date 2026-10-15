// Package token creates random secrets for sessions and emailed links, and
// guild invite codes.
package token

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

// Random returns 32 random bytes, base64url-encoded without padding.
type Random struct{}

func (Random) NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NewInviteCode returns domain.InviteCodeLen characters from the invite
// alphabet. The alphabet has 32 symbols, so each random byte maps to one
// without bias.
func (Random) NewInviteCode() (string, error) {
	b := make([]byte, domain.InviteCodeLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate invite code: %w", err)
	}
	for i := range b {
		b[i] = domain.InviteCodeAlphabet[int(b[i])%len(domain.InviteCodeAlphabet)]
	}
	return string(b), nil
}
