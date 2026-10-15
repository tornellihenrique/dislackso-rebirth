// Package instance is the instance admin's control panel: settings, users
// and reset links.
package instance

import (
	"context"
	"errors"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/settings"
)

type Settings interface {
	Get(ctx context.Context) (domain.InstanceSettings, error)
	Update(ctx context.Context, p settings.Patch) (domain.InstanceSettings, error)
	SMTPPasswordSet(ctx context.Context) (bool, error)
}

type ResetLinks interface {
	IssuePasswordResetLink(ctx context.Context, userID domain.ID) (string, time.Time, error)
}

type TestEmails interface {
	Test(ctx context.Context, to, locale string) error
}

type Deps struct {
	Users      ports.UserRepo
	Sessions   ports.SessionRepo
	Tx         ports.TxRunner
	Clock      ports.Clock
	Settings   Settings
	ResetLinks ResetLinks
	Emails     TestEmails
}

type Service struct {
	d      Deps
	policy domain.Policy
}

func New(d Deps) *Service { return &Service{d: d} }

type SettingsView struct {
	Settings        domain.InstanceSettings
	SMTPPasswordSet bool
}

func (s *Service) Settings(ctx context.Context, actor domain.Actor) (SettingsView, error) {
	if err := s.policy.ManageInstance(actor); err != nil {
		return SettingsView{}, err
	}
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return SettingsView{}, err
	}
	return s.view(ctx, st)
}

func (s *Service) UpdateSettings(ctx context.Context, actor domain.Actor, p settings.Patch) (SettingsView, error) {
	if err := s.policy.ManageInstance(actor); err != nil {
		return SettingsView{}, err
	}
	p.MarkSetupComplete = false
	st, err := s.d.Settings.Update(ctx, p)
	if err != nil {
		return SettingsView{}, err
	}
	return s.view(ctx, st)
}

func (s *Service) view(ctx context.Context, st domain.InstanceSettings) (SettingsView, error) {
	set, err := s.d.Settings.SMTPPasswordSet(ctx)
	if err != nil {
		return SettingsView{}, err
	}
	return SettingsView{Settings: st, SMTPPasswordSet: set}, nil
}

// SendTestEmail sends a test message with the saved settings.
func (s *Service) SendTestEmail(ctx context.Context, actor domain.Actor, client domain.ClientInfo, to string) error {
	if err := s.policy.ManageInstance(actor); err != nil {
		return err
	}
	to = domain.NormalizeEmail(to)
	if v := domain.ValidateEmail("to", to); v != nil {
		return domain.Invalid(*v)
	}
	if err := s.d.Emails.Test(ctx, to, client.Locale); err != nil {
		return &domain.Error{Kind: domain.ErrFailedPrecondition, Reason: domain.ReasonEmailDeliveryFailed, Message: err.Error()}
	}
	return nil
}

func (s *Service) Users(ctx context.Context, actor domain.Actor) ([]domain.User, error) {
	if err := s.policy.ManageInstance(actor); err != nil {
		return nil, err
	}
	return s.d.Users.ListUsers(ctx)
}

// SetUserDisabled blocks or unblocks an account. Disabling signs it out everywhere.
func (s *Service) SetUserDisabled(ctx context.Context, actor domain.Actor, userID domain.ID, disabled bool) (domain.User, error) {
	return s.changeUser(ctx, actor, userID, func(ctx context.Context, u *domain.User, now time.Time) error {
		if !disabled {
			u.DisabledAt = nil
			return nil
		}
		if u.DisabledAt == nil {
			u.DisabledAt = &now
		}
		_, err := s.d.Sessions.RevokeUserSessions(ctx, u.ID, 0, now)
		return err
	})
}

func (s *Service) SetUserInstanceAdmin(ctx context.Context, actor domain.Actor, userID domain.ID, admin bool) (domain.User, error) {
	return s.changeUser(ctx, actor, userID, func(_ context.Context, u *domain.User, _ time.Time) error {
		u.IsInstanceAdmin = admin
		return nil
	})
}

// changeUser applies change to another user's account. Admins cannot target
// themselves, so an instance never loses its last admin by accident.
func (s *Service) changeUser(ctx context.Context, actor domain.Actor, userID domain.ID, change func(context.Context, *domain.User, time.Time) error) (domain.User, error) {
	if err := s.policy.ManageInstance(actor); err != nil {
		return domain.User{}, err
	}
	if userID == actor.UserID {
		return domain.User{}, domain.Forbidden(domain.ReasonCannotTargetSelf)
	}
	var user domain.User
	err := s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		user, err = s.d.Users.UserByID(ctx, userID)
		if errors.Is(err, domain.ErrNotFound) {
			return domain.NotFound(domain.ReasonUserNotFound)
		}
		if err != nil {
			return err
		}
		now := s.d.Clock.Now()
		if err := change(ctx, &user, now); err != nil {
			return err
		}
		user.UpdatedAt = now
		return s.d.Users.UpdateUser(ctx, user)
	})
	return user, err
}

func (s *Service) CreatePasswordResetLink(ctx context.Context, actor domain.Actor, userID domain.ID) (string, time.Time, error) {
	if err := s.policy.ManageInstance(actor); err != nil {
		return "", time.Time{}, err
	}
	return s.d.ResetLinks.IssuePasswordResetLink(ctx, userID)
}
