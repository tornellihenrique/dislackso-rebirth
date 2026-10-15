// Package invite manages invites. Instance invites let one person register
// while registration is invite-only; guild invites join a guild, and also
// let a newcomer register.
package invite

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/internal/access"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/internal/emailtoken"
)

const (
	DefaultInstanceInviteTTL = 7 * 24 * time.Hour
	MaxInstanceInviteTTL     = 30 * 24 * time.Hour
)

type SettingsReader interface {
	Get(ctx context.Context) (domain.InstanceSettings, error)
}

type Deps struct {
	Tokens       ports.EmailTokenRepo
	Users        ports.UserRepo
	Guilds       ports.GuildRepo
	Members      ports.MemberRepo
	Channels     ports.ChannelRepo
	Invites      ports.InviteRepo
	Bans         ports.BanRepo
	Audit        ports.AuditRepo
	Tx           ports.TxRunner
	TokenGen     ports.TokenGen
	Codes        ports.CodeGen
	IDs          ports.IDGen
	Clock        ports.Clock
	Limiter      ports.RateLimiter
	Events       ports.EventPublisher
	Settings     SettingsReader
	Presence     access.Presences
	Voice        access.VoiceStates
	Verification access.Verification
}

type Service struct {
	d       Deps
	tokens  emailtoken.Store
	access  access.Guilds
	auditor access.Auditor
	policy  domain.Policy
}

func New(d Deps) *Service {
	return &Service{
		d:      d,
		tokens: emailtoken.Store{Repo: d.Tokens, Tokens: d.TokenGen, IDs: d.IDs, Clock: d.Clock},
		access: access.Guilds{
			Guilds: d.Guilds, Members: d.Members, Channels: d.Channels, Users: d.Users,
			Presence: d.Presence, Verification: d.Verification, Voice: d.Voice,
		},
		auditor: access.Auditor{Repo: d.Audit, IDs: d.IDs, Clock: d.Clock, Events: d.Events},
	}
}

// InstanceInvite is an invite as shown to admins. The code itself is only
// known when the invite is created.
type InstanceInvite struct {
	Token     domain.EmailToken
	CreatedBy *domain.User
}

type CreatedInvite struct {
	Invite InstanceInvite
	Code   string
	URL    string
}

// CreateInstanceInvite issues an invite; ttl zero selects the default.
func (s *Service) CreateInstanceInvite(ctx context.Context, actor domain.Actor, ttl time.Duration) (CreatedInvite, error) {
	if err := s.policy.ManageInstance(actor); err != nil {
		return CreatedInvite{}, err
	}
	switch {
	case ttl == 0:
		ttl = DefaultInstanceInviteTTL
	case ttl < time.Minute || ttl > MaxInstanceInviteTTL:
		return CreatedInvite{}, domain.Invalid(domain.FieldViolation{Field: "expires_in_ms", Reason: domain.ReasonFieldInvalid})
	}
	code, tok, err := s.tokens.Issue(ctx, domain.TokenInstanceInvite, actor.UserID, ttl, "")
	if err != nil {
		return CreatedInvite{}, err
	}
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return CreatedInvite{}, err
	}
	creator, err := s.creator(ctx, actor.UserID)
	if err != nil {
		return CreatedInvite{}, err
	}
	return CreatedInvite{
		Invite: InstanceInvite{Token: tok, CreatedBy: creator},
		Code:   code,
		URL:    st.PublicURL + "/register?invite=" + url.QueryEscape(code),
	}, nil
}

func (s *Service) ListInstanceInvites(ctx context.Context, actor domain.Actor) ([]InstanceInvite, error) {
	if err := s.policy.ManageInstance(actor); err != nil {
		return nil, err
	}
	toks, err := s.d.Tokens.EmailTokensByPurpose(ctx, domain.TokenInstanceInvite)
	if err != nil {
		return nil, err
	}
	creators := map[domain.ID]*domain.User{}
	out := make([]InstanceInvite, len(toks))
	for i, t := range toks {
		c, ok := creators[t.UserID]
		if !ok {
			if c, err = s.creator(ctx, t.UserID); err != nil {
				return nil, err
			}
			creators[t.UserID] = c
		}
		out[i] = InstanceInvite{Token: t, CreatedBy: c}
	}
	return out, nil
}

func (s *Service) RevokeInstanceInvite(ctx context.Context, actor domain.Actor, id domain.ID) error {
	if err := s.policy.ManageInstance(actor); err != nil {
		return err
	}
	err := s.d.Tokens.DeleteEmailToken(ctx, id, domain.TokenInstanceInvite)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.NotFound(domain.ReasonInviteNotFound)
	}
	return err
}

// ClaimForRegistration redeems an invite for a new account: an instance
// invite, or a guild invite that the account then joins.
func (s *Service) ClaimForRegistration(ctx context.Context, code string) (func(context.Context, domain.ID) error, error) {
	if guildCode := domain.NormalizeInviteCode(code); domain.IsGuildInviteCode(guildCode) {
		return s.claimGuildInvite(ctx, guildCode)
	}
	if _, err := s.tokens.Redeem(ctx, domain.TokenInstanceInvite, code); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.Invalid(domain.FieldViolation{Field: "invite_code", Reason: domain.ReasonFieldInvalid})
		}
		return nil, err
	}
	return func(context.Context, domain.ID) error { return nil }, nil
}

func (s *Service) creator(ctx context.Context, id domain.ID) (*domain.User, error) {
	u, err := s.d.Users.UserByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
