// Package account manages the signed-in user's own account: profile,
// password, status preference and sessions.
package account

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

var limitChangePassword = ports.Limit{Burst: 5, Every: time.Minute}

// Verification tells whether accounts must confirm their email first.
type Verification interface {
	VerificationRequired(ctx context.Context) bool
}

type Deps struct {
	Users        ports.UserRepo
	Sessions     ports.SessionRepo
	Tx           ports.TxRunner
	Hasher       ports.PasswordHasher
	Clock        ports.Clock
	Limiter      ports.RateLimiter
	Verification Verification
}

type Service struct {
	d Deps
}

func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) Me(ctx context.Context, actor domain.Actor) (domain.User, error) {
	return s.d.Users.UserByID(ctx, actor.UserID)
}

func (s *Service) UpdateProfile(ctx context.Context, actor domain.Actor, displayName string) (domain.User, error) {
	if err := s.requireVerified(ctx, actor); err != nil {
		return domain.User{}, err
	}
	displayName = domain.NormalizeDisplayName(displayName)
	if v := domain.ValidateDisplayName("display_name", displayName); v != nil {
		return domain.User{}, domain.Invalid(*v)
	}
	return s.update(ctx, actor, func(u *domain.User) { u.DisplayName = displayName })
}

func (s *Service) SetStatus(ctx context.Context, actor domain.Actor, pref domain.StatusPref, custom string) (domain.User, error) {
	if err := s.requireVerified(ctx, actor); err != nil {
		return domain.User{}, err
	}
	custom = strings.TrimSpace(custom)
	checks := []*domain.FieldViolation{domain.ValidateCustomStatus("custom_status", custom)}
	if !pref.Valid() {
		checks = append(checks, &domain.FieldViolation{Field: "status", Reason: domain.ReasonFieldInvalidFormat})
	}
	if err := domain.Violations(checks...); err != nil {
		return domain.User{}, err
	}
	return s.update(ctx, actor, func(u *domain.User) { u.StatusPref, u.CustomStatus = pref, custom })
}

// ChangePassword verifies the current password, sets the new one and signs
// out every other session.
func (s *Service) ChangePassword(ctx context.Context, actor domain.Actor, current, next string) error {
	if err := s.requireVerified(ctx, actor); err != nil {
		return err
	}
	if ok, retry := s.d.Limiter.Allow("password:user:"+actor.UserID.String(), limitChangePassword); !ok {
		return domain.RateLimited(retry)
	}
	if v := domain.ValidatePassword("new_password", next); v != nil {
		return domain.Invalid(*v)
	}
	user, err := s.d.Users.UserByID(ctx, actor.UserID)
	if err != nil {
		return err
	}
	ok, err := s.d.Hasher.Verify(user.PasswordHash, current)
	if err != nil {
		return fmt.Errorf("verify password of user %d: %w", user.ID, err)
	}
	if !ok {
		return domain.Invalid(domain.FieldViolation{Field: "current_password", Reason: domain.ReasonFieldIncorrect})
	}
	hash, err := s.d.Hasher.Hash(next)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		now := s.d.Clock.Now()
		user.PasswordHash, user.UpdatedAt = hash, now
		if err := s.d.Users.UpdateUser(ctx, user); err != nil {
			return err
		}
		_, err := s.d.Sessions.RevokeUserSessions(ctx, user.ID, actor.SessionID, now)
		return err
	})
}

func (s *Service) Sessions(ctx context.Context, actor domain.Actor) ([]domain.Session, error) {
	return s.d.Sessions.ActiveSessions(ctx, actor.UserID, s.d.Clock.Now())
}

func (s *Service) RevokeSession(ctx context.Context, actor domain.Actor, id domain.ID) error {
	err := s.d.Sessions.RevokeSession(ctx, actor.UserID, id, s.d.Clock.Now())
	if errors.Is(err, domain.ErrNotFound) {
		return domain.NotFound(domain.ReasonSessionNotFound)
	}
	return err
}

func (s *Service) RevokeOtherSessions(ctx context.Context, actor domain.Actor) (int, error) {
	return s.d.Sessions.RevokeUserSessions(ctx, actor.UserID, actor.SessionID, s.d.Clock.Now())
}

func (s *Service) update(ctx context.Context, actor domain.Actor, change func(*domain.User)) (domain.User, error) {
	var user domain.User
	err := s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if user, err = s.d.Users.UserByID(ctx, actor.UserID); err != nil {
			return err
		}
		change(&user)
		user.UpdatedAt = s.d.Clock.Now()
		return s.d.Users.UpdateUser(ctx, user)
	})
	return user, err
}

func (s *Service) requireVerified(ctx context.Context, actor domain.Actor) error {
	if !actor.EmailVerified && s.d.Verification.VerificationRequired(ctx) {
		return domain.Precondition(domain.ReasonEmailNotVerified)
	}
	return nil
}
