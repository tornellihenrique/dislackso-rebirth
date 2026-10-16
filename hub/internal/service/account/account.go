// Package account manages the signed-in user's own account: profile,
// password, status preference, sessions and deletion.
package account

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/guild"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/internal/access"
)

var limitChangePassword = ports.Limit{Burst: 5, Every: time.Minute}

// Verification tells whether accounts must confirm their email first.
type Verification interface {
	VerificationRequired(ctx context.Context) bool
}

// StatusSink applies a new status preference to the user's live presence.
type StatusSink interface {
	SetPreference(ctx context.Context, userID domain.ID, pref domain.StatusPref, custom string)
}

type Deps struct {
	Users        ports.UserRepo
	Sessions     ports.SessionRepo
	Members      ports.MemberRepo
	Guilds       ports.GuildRepo
	Files        ports.FileRepo
	Tx           ports.TxRunner
	Hasher       ports.PasswordHasher
	Clock        ports.Clock
	Limiter      ports.RateLimiter
	Events       ports.EventPublisher
	Presence     StatusSink
	Verification Verification
}

type Service struct {
	d Deps
}

func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) Me(ctx context.Context, actor domain.Actor) (domain.User, error) {
	return s.d.Users.UserByID(ctx, actor.UserID)
}

// ProfilePatch changes the fields that are set; a zero AvatarFileID removes the avatar.
type ProfilePatch struct {
	DisplayName  *string
	AvatarFileID *domain.ID
}

// UpdateProfile changes the public profile, which everyone sharing a guild
// with the user sees change.
func (s *Service) UpdateProfile(ctx context.Context, actor domain.Actor, p ProfilePatch) (domain.User, error) {
	if err := s.requireVerified(ctx, actor); err != nil {
		return domain.User{}, err
	}
	var name string
	if p.DisplayName != nil {
		name = domain.NormalizeDisplayName(*p.DisplayName)
		if v := domain.ValidateDisplayName("display_name", name); v != nil {
			return domain.User{}, domain.Invalid(*v)
		}
	}
	if p.AvatarFileID != nil && *p.AvatarFileID != 0 {
		if err := access.OwnFile(ctx, s.d.Files, actor, *p.AvatarFileID, domain.FileAvatar, "avatar_file_id"); err != nil {
			return domain.User{}, err
		}
	}
	return s.update(ctx, actor, true, func(u *domain.User) {
		if p.DisplayName != nil {
			u.DisplayName = name
		}
		if p.AvatarFileID != nil {
			u.AvatarFileID = *p.AvatarFileID
		}
	})
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
	u, err := s.update(ctx, actor, false, func(u *domain.User) { u.StatusPref, u.CustomStatus = pref, custom })
	if err != nil {
		return domain.User{}, err
	}
	s.d.Presence.SetPreference(ctx, u.ID, u.StatusPref, u.CustomStatus)
	return u, nil
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
	if err := s.checkPassword(user, current, "current_password"); err != nil {
		return err
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

// update changes the user and tells their other sessions; public changes
// also reach everyone sharing a guild with them.
func (s *Service) update(ctx context.Context, actor domain.Actor, public bool, change func(*domain.User)) (domain.User, error) {
	var user domain.User
	err := s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if user, err = s.d.Users.UserByID(ctx, actor.UserID); err != nil {
			return err
		}
		change(&user)
		user.UpdatedAt = s.d.Clock.Now()
		if err := s.d.Users.UpdateUser(ctx, user); err != nil {
			return err
		}
		evs := []events.Event{{Topics: []events.Topic{events.UserTopic(user.ID)}, Payload: events.MeUpdated{User: user}}}
		if public {
			topics, err := access.GuildTopics(ctx, s.d.Members, user.ID)
			if err != nil {
				return err
			}
			evs = append(evs, events.Event{Topics: topics, Payload: events.UserUpdated{User: user}})
		}
		s.d.Events.Publish(ctx, evs...)
		return nil
	})
	return user, err
}

func (s *Service) requireVerified(ctx context.Context, actor domain.Actor) error {
	if !actor.EmailVerified && s.d.Verification.VerificationRequired(ctx) {
		return domain.Precondition(domain.ReasonEmailNotVerified)
	}
	return nil
}

// DeleteAccount turns the account into an anonymous "Deleted User": it
// leaves every server, its sessions end and its username and email become
// free. Servers it owns must be handed over or deleted first, and the last
// instance admin cannot leave the instance without one.
func (s *Service) DeleteAccount(ctx context.Context, actor domain.Actor, password string) error {
	if ok, retry := s.d.Limiter.Allow("password:user:"+actor.UserID.String(), limitChangePassword); !ok {
		return domain.RateLimited(retry)
	}
	user, err := s.d.Users.UserByID(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if err := s.checkPassword(user, password, "password"); err != nil {
		return err
	}
	owned, err := s.d.Guilds.CountGuildsOwnedBy(ctx, user.ID)
	if err != nil {
		return err
	}
	if owned > 0 {
		return domain.Precondition(domain.ReasonOwnsGuilds)
	}
	if user.IsInstanceAdmin {
		if err := s.otherAdminExists(ctx, user.ID); err != nil {
			return err
		}
	}
	return s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		memberships, err := s.d.Members.UserMemberships(ctx, user.ID)
		if err != nil {
			return err
		}
		for _, m := range memberships {
			if err := s.d.Members.RemoveMember(ctx, m.GuildID, user.ID); err != nil {
				return err
			}
			guild.RemovedMember(ctx, s.d.Events, m.GuildID, user.ID, events.ReasonLeft)
		}
		now := s.d.Clock.Now()
		if _, err := s.d.Sessions.RevokeUserSessions(ctx, user.ID, 0, now); err != nil {
			return err
		}
		return s.d.Users.DeleteUser(ctx, user.ID, now)
	})
}

func (s *Service) otherAdminExists(ctx context.Context, self domain.ID) error {
	users, err := s.d.Users.ListUsers(ctx)
	if err != nil {
		return err
	}
	for _, u := range users {
		if u.IsInstanceAdmin && u.ID != self && !u.Disabled() {
			return nil
		}
	}
	return domain.Precondition(domain.ReasonLastInstanceAdmin)
}

func (s *Service) checkPassword(user domain.User, password, field string) error {
	ok, err := s.d.Hasher.Verify(user.PasswordHash, password)
	if err != nil {
		return fmt.Errorf("verify password of user %d: %w", user.ID, err)
	}
	if !ok {
		return domain.Invalid(domain.FieldViolation{Field: field, Reason: domain.ReasonFieldIncorrect})
	}
	return nil
}
