package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

const changeEmailTokenTTL = 24 * time.Hour

var limitEmailChangeUser = ports.Limit{Burst: 3, Every: 10 * time.Minute}

// EmailChange is the outcome of RequestEmailChange: Sent when a link went to
// the new address, or the user with the new address already in place.
type EmailChange struct {
	Sent bool
	User domain.User
}

// RequestEmailChange checks the password and emails a confirmation link to
// the new address. Without email the address changes at once, unconfirmed.
// Unverified accounts may use it too: a mistyped address is the usual reason.
func (s *Service) RequestEmailChange(ctx context.Context, client domain.ClientInfo, actor domain.Actor, newEmail, password string) (EmailChange, error) {
	if err := s.allow("email-change:user:"+actor.UserID.String(), limitEmailChangeUser); err != nil {
		return EmailChange{}, err
	}
	newEmail = domain.NormalizeEmail(newEmail)
	if v := domain.ValidateEmail("new_email", newEmail); v != nil {
		return EmailChange{}, domain.Invalid(*v)
	}
	user, err := s.d.Users.UserByID(ctx, actor.UserID)
	if err != nil {
		return EmailChange{}, err
	}
	ok, err := s.d.Hasher.Verify(user.PasswordHash, password)
	if err != nil {
		return EmailChange{}, fmt.Errorf("verify password of user %d: %w", user.ID, err)
	}
	if !ok {
		return EmailChange{}, domain.Invalid(domain.FieldViolation{Field: "password", Reason: domain.ReasonFieldIncorrect})
	}
	if strings.EqualFold(user.Email, newEmail) {
		return EmailChange{}, domain.Invalid(domain.FieldViolation{Field: "new_email", Reason: domain.ReasonFieldUnchanged})
	}
	if err := s.emailFree(ctx, newEmail, user.ID); err != nil {
		return EmailChange{}, err
	}

	if !s.VerificationRequired(ctx) {
		err := s.d.Tx.InTx(ctx, func(ctx context.Context) error {
			return s.setEmail(ctx, &user, newEmail, nil)
		})
		return EmailChange{User: user}, err
	}
	var raw string
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Tokens.ExpireUserTokens(ctx, user.ID, domain.TokenChangeEmail, s.d.Clock.Now()); err != nil {
			return err
		}
		var err error
		raw, _, err = s.tokens.Issue(ctx, domain.TokenChangeEmail, user.ID, changeEmailTokenTTL, newEmail)
		return err
	})
	if err != nil {
		return EmailChange{}, err
	}
	link, err := s.link(ctx, "/confirm-email", raw)
	if err != nil {
		return EmailChange{}, err
	}
	if err := s.d.Emails.ConfirmEmailChange(ctx, newEmail, user.DisplayName, client.Locale, link); err != nil {
		s.d.Logger.WarnContext(ctx, "send email change confirmation", slog.String("user_id", user.ID.String()), slog.Any("error", err))
		return EmailChange{}, &domain.Error{Kind: domain.ErrFailedPrecondition, Reason: domain.ReasonEmailDeliveryFailed}
	}
	return EmailChange{Sent: true, User: user}, nil
}

// ConfirmEmailChange moves the account to the address the link was sent to,
// which the click also verifies.
func (s *Service) ConfirmEmailChange(ctx context.Context, client domain.ClientInfo, raw string) error {
	if err := s.allow("token:ip:"+client.IP.String(), limitTokenIP); err != nil {
		return err
	}
	return s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		tok, err := s.tokens.Redeem(ctx, domain.TokenChangeEmail, raw)
		if err != nil {
			return err
		}
		user, err := s.d.Users.UserByID(ctx, tok.UserID)
		if err != nil {
			return err
		}
		if err := s.emailFree(ctx, tok.Payload, user.ID); err != nil {
			return err
		}
		now := s.d.Clock.Now()
		return s.setEmail(ctx, &user, tok.Payload, &now)
	})
}

// setEmail changes the address inside a transaction. Links sent to the old
// address stop working.
func (s *Service) setEmail(ctx context.Context, user *domain.User, email string, verifiedAt *time.Time) error {
	now := s.d.Clock.Now()
	user.Email, user.EmailVerifiedAt, user.UpdatedAt = email, verifiedAt, now
	if err := s.d.Users.UpdateUser(ctx, *user); err != nil {
		return err
	}
	for _, p := range []domain.TokenPurpose{domain.TokenChangeEmail, domain.TokenVerifyEmail, domain.TokenPasswordReset} {
		if err := s.d.Tokens.ExpireUserTokens(ctx, user.ID, p, now); err != nil {
			return err
		}
	}
	s.d.Events.Publish(ctx, events.Event{Topics: []events.Topic{events.UserTopic(user.ID)}, Payload: events.MeUpdated{User: *user}})
	return nil
}

func (s *Service) emailFree(ctx context.Context, email string, self domain.ID) error {
	other, err := s.d.Users.UserByEmail(ctx, email)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return nil
	case err != nil:
		return err
	case other.ID == self:
		return nil
	}
	return domain.Conflict("new_email")
}
