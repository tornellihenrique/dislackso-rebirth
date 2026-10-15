// Package token creates random secrets for sessions and emailed links.
package token

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
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
