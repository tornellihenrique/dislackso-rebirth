// Package emailtoken issues and redeems the single-use secrets behind email
// links and invites. Only the hash is stored.
package emailtoken

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

type Store struct {
	Repo   ports.EmailTokenRepo
	Tokens ports.TokenGen
	IDs    ports.IDGen
	Clock  ports.Clock
}

// Issue creates a token and returns the raw secret to send to the user.
func (s Store) Issue(ctx context.Context, purpose domain.TokenPurpose, userID domain.ID, ttl time.Duration, payload string) (string, domain.EmailToken, error) {
	raw, err := s.Tokens.NewToken()
	if err != nil {
		return "", domain.EmailToken{}, err
	}
	now := s.Clock.Now()
	t := domain.EmailToken{
		ID:        s.IDs.NewID(),
		UserID:    userID,
		Purpose:   purpose,
		TokenHash: domain.HashToken(raw),
		Payload:   payload,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}
	if err := s.Repo.CreateEmailToken(ctx, t); err != nil {
		return "", domain.EmailToken{}, fmt.Errorf("issue %s token: %w", purpose, err)
	}
	return raw, t, nil
}

// Redeem marks the token used and returns it. Unknown, expired, used and
// wrong-purpose tokens all fail the same way.
func (s Store) Redeem(ctx context.Context, purpose domain.TokenPurpose, raw string) (domain.EmailToken, error) {
	invalid := domain.NotFound(domain.ReasonTokenInvalid)
	if raw == "" {
		return domain.EmailToken{}, invalid
	}
	t, err := s.Repo.EmailTokenByHash(ctx, domain.HashToken(raw))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.EmailToken{}, invalid
	}
	if err != nil {
		return domain.EmailToken{}, err
	}
	now := s.Clock.Now()
	if t.Purpose != purpose || !t.Usable(now) {
		return domain.EmailToken{}, invalid
	}
	if err := s.Repo.UseEmailToken(ctx, t.ID, now); errors.Is(err, domain.ErrNotFound) {
		return domain.EmailToken{}, invalid
	} else if err != nil {
		return domain.EmailToken{}, err
	}
	t.UsedAt = &now
	return t, nil
}
