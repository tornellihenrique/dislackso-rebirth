package invite

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/internal/access"
)

var (
	limitPreviewIP  = ports.Limit{Burst: 30, Every: 2 * time.Second}
	limitAcceptUser = ports.Limit{Burst: 10, Every: 30 * time.Second}
	limitCreateUser = ports.Limit{Burst: 10, Every: 30 * time.Second}
)

// codeAttempts bounds retries when a random code is already taken; with 32^8
// codes a second attempt is already unlikely.
const codeAttempts = 5

// CreateGuildInvite issues an invite link; maxAge zero never expires and
// maxUses zero is unlimited.
func (s *Service) CreateGuildInvite(ctx context.Context, actor domain.Actor, guildID domain.ID, maxAge time.Duration, maxUses int) (domain.InviteView, error) {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return domain.InviteView{}, err
	}
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return domain.InviteView{}, err
	}
	if err := s.policy.CreateInvite(m, st.MembersCanInvite); err != nil {
		return domain.InviteView{}, err
	}
	if err := domain.ValidateInviteOptions(maxAge, maxUses); err != nil {
		return domain.InviteView{}, err
	}
	if ok, retry := s.d.Limiter.Allow("invite:create:user:"+actor.UserID.String(), limitCreateUser); !ok {
		return domain.InviteView{}, domain.RateLimited(retry)
	}

	now := s.d.Clock.Now()
	inv := domain.Invite{GuildID: guildID, CreatedBy: actor.UserID, MaxUses: maxUses, CreatedAt: now}
	if maxAge > 0 {
		exp := now.Add(maxAge)
		inv.ExpiresAt = &exp
	}
	var view domain.InviteView
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.insertWithFreshCode(ctx, &inv); err != nil {
			return err
		}
		creator, err := s.creator(ctx, actor.UserID)
		if err != nil {
			return err
		}
		view = domain.InviteView{Invite: inv, Creator: creator, URL: inviteURL(st, inv.Code)}
		if err := s.auditor.Record(ctx, access.Entry{
			GuildID: guildID, ActorID: actor.UserID, Action: domain.AuditInviteCreate,
			TargetType: domain.AuditTargetInvite, TargetID: inv.Code,
			Data: map[string]string{
				"max_uses":   strconv.Itoa(maxUses),
				"max_age_ms": strconv.FormatInt(maxAge.Milliseconds(), 10),
			},
		}); err != nil {
			return err
		}
		s.d.Events.Publish(ctx, events.Event{
			Topics:  inviteTopics(guildID, actor.UserID),
			Payload: events.InviteCreated{Invite: view},
		})
		return nil
	})
	return view, err
}

func (s *Service) insertWithFreshCode(ctx context.Context, inv *domain.Invite) error {
	for range codeAttempts {
		code, err := s.d.Codes.NewInviteCode()
		if err != nil {
			return err
		}
		inv.Code = code
		err = s.d.Invites.CreateInvite(ctx, *inv)
		if !errors.Is(err, domain.ErrConflict) {
			return err
		}
	}
	return errors.New("create invite: no free code after several attempts")
}

// inviteTopics reaches the guild's admins and the invite's creator, who may
// see their own invites.
func inviteTopics(guildID, creatorID domain.ID) []events.Topic {
	return []events.Topic{events.GuildAdminsTopic(guildID), events.UserTopic(creatorID)}
}

func inviteURL(st domain.InstanceSettings, code string) string {
	return st.PublicURL + "/invite/" + code
}

// ListGuildInvites returns the usable invites: all of them for owners and
// admins, the caller's own for members.
func (s *Service) ListGuildInvites(ctx context.Context, actor domain.Actor, guildID domain.ID) ([]domain.InviteView, error) {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return nil, err
	}
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	all, err := s.d.Invites.UsableInvites(ctx, guildID, s.d.Clock.Now())
	if err != nil {
		return nil, err
	}
	everyone := s.policy.Guild(m, domain.ActManageInvites) == nil
	creators := map[domain.ID]*domain.User{}
	var out []domain.InviteView
	for _, inv := range all {
		if !everyone && inv.CreatedBy != actor.UserID {
			continue
		}
		c, ok := creators[inv.CreatedBy]
		if !ok {
			if c, err = s.creator(ctx, inv.CreatedBy); err != nil {
				return nil, err
			}
			creators[inv.CreatedBy] = c
		}
		out = append(out, domain.InviteView{Invite: inv, Creator: c, URL: inviteURL(st, inv.Code)})
	}
	return out, nil
}

func (s *Service) RevokeGuildInvite(ctx context.Context, actor domain.Actor, code string) error {
	code = domain.NormalizeInviteCode(code)
	notFound := domain.NotFound(domain.ReasonInviteNotFound)
	inv, err := s.d.Invites.InviteByCode(ctx, code)
	if errors.Is(err, domain.ErrNotFound) {
		return notFound
	}
	if err != nil {
		return err
	}
	m, err := s.access.Member(ctx, actor, inv.GuildID)
	if errors.Is(err, domain.ErrNotFound) {
		return notFound
	}
	if err != nil {
		return err
	}
	if err := s.policy.RevokeInvite(m, inv); err != nil {
		return err
	}
	return s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		err := s.d.Invites.RevokeInvite(ctx, code, s.d.Clock.Now())
		if errors.Is(err, domain.ErrNotFound) {
			return notFound
		}
		if err != nil {
			return err
		}
		if err := s.auditor.Record(ctx, access.Entry{
			GuildID: inv.GuildID, ActorID: actor.UserID, Action: domain.AuditInviteRevoke,
			TargetType: domain.AuditTargetInvite, TargetID: code,
		}); err != nil {
			return err
		}
		s.d.Events.Publish(ctx, events.Event{
			Topics:  inviteTopics(inv.GuildID, inv.CreatedBy),
			Payload: events.InviteDeleted{GuildID: inv.GuildID, Code: code},
		})
		return nil
	})
}

// Preview is what the invite landing page shows before joining.
type Preview struct {
	Invite      domain.Invite
	Guild       domain.Guild
	MemberCount int
	OnlineCount int
	Inviter     *domain.User
	// Member is set when the caller is signed in and already in the guild.
	Member bool
}

// PreviewGuildInvite works signed out (actor nil). Banned users get
// Forbidden, so they cannot even see who is in the guild.
func (s *Service) PreviewGuildInvite(ctx context.Context, client domain.ClientInfo, actor *domain.Actor, code string) (Preview, error) {
	if ok, retry := s.d.Limiter.Allow("invite:preview:ip:"+client.IP.String(), limitPreviewIP); !ok {
		return Preview{}, domain.RateLimited(retry)
	}
	inv, err := s.usableInvite(ctx, domain.NormalizeInviteCode(code))
	if err != nil {
		return Preview{}, err
	}
	g, err := s.d.Guilds.GuildByID(ctx, inv.GuildID)
	if err != nil {
		return Preview{}, err
	}
	p := Preview{Invite: inv, Guild: g}
	if actor != nil {
		if err := s.notBanned(ctx, g.ID, actor.UserID); err != nil {
			return Preview{}, err
		}
		if _, err := s.d.Members.MemberByID(ctx, g.ID, actor.UserID); err == nil {
			p.Member = true
		} else if !errors.Is(err, domain.ErrNotFound) {
			return Preview{}, err
		}
	}
	members, err := s.d.Members.GuildMembers(ctx, g.ID)
	if err != nil {
		return Preview{}, err
	}
	ids := make([]domain.ID, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	p.MemberCount = len(members)
	for _, pr := range s.d.Presence.Presences(ids) {
		if pr.Status != domain.PresenceOffline {
			p.OnlineCount++
		}
	}
	if p.Inviter, err = s.creator(ctx, inv.CreatedBy); err != nil {
		return Preview{}, err
	}
	return p, nil
}

// AcceptGuildInvite joins the guild. Accepting while already a member
// returns the guild without counting a use.
func (s *Service) AcceptGuildInvite(ctx context.Context, actor domain.Actor, code string) (domain.GuildState, error) {
	if err := s.access.Verified(ctx, actor); err != nil {
		return domain.GuildState{}, err
	}
	if ok, retry := s.d.Limiter.Allow("invite:accept:user:"+actor.UserID.String(), limitAcceptUser); !ok {
		return domain.GuildState{}, domain.RateLimited(retry)
	}
	code = domain.NormalizeInviteCode(code)
	var state domain.GuildState
	err := s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		inv, err := s.usableInvite(ctx, code)
		if err != nil {
			return err
		}
		g, err := s.d.Guilds.GuildByID(ctx, inv.GuildID)
		if err != nil {
			return err
		}
		if _, err := s.d.Members.MemberByID(ctx, g.ID, actor.UserID); err == nil {
			state, err = s.access.State(ctx, g)
			return err
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		if err := s.notBanned(ctx, g.ID, actor.UserID); err != nil {
			return err
		}
		m, err := s.join(ctx, inv, actor.UserID)
		if err != nil {
			return err
		}
		if state, err = s.access.State(ctx, g); err != nil {
			return err
		}
		user, err := s.d.Users.UserByID(ctx, actor.UserID)
		if err != nil {
			return err
		}
		s.d.Events.Publish(ctx,
			events.Event{
				Topics:  []events.Topic{events.UserTopic(actor.UserID)},
				Payload: events.GuildCreated{UserID: actor.UserID, State: state},
			},
			s.memberAdded(m, user),
		)
		return nil
	})
	return state, err
}

// claimGuildInvite lets a newcomer register with a guild invite and join
// the guild once the account exists.
func (s *Service) claimGuildInvite(ctx context.Context, code string) (func(context.Context, domain.ID) error, error) {
	inv, err := s.usableInvite(ctx, code)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.Invalid(domain.FieldViolation{Field: "invite_code", Reason: domain.ReasonFieldInvalid})
	}
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, userID domain.ID) error {
		m, err := s.join(ctx, inv, userID)
		if err != nil {
			return err
		}
		user, err := s.d.Users.UserByID(ctx, userID)
		if err != nil {
			return err
		}
		s.d.Events.Publish(ctx, s.memberAdded(m, user))
		return nil
	}, nil
}

func (s *Service) join(ctx context.Context, inv domain.Invite, userID domain.ID) (domain.Member, error) {
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return domain.Member{}, err
	}
	n, err := s.d.Members.CountMembers(ctx, inv.GuildID)
	if err != nil {
		return domain.Member{}, err
	}
	if n >= st.Limits.MaxGuildMembers {
		return domain.Member{}, domain.Precondition(domain.ReasonGuildFull)
	}
	now := s.d.Clock.Now()
	if err := s.d.Invites.UseInvite(ctx, inv.Code, now); errors.Is(err, domain.ErrNotFound) {
		return domain.Member{}, domain.NotFound(domain.ReasonInviteNotFound)
	} else if err != nil {
		return domain.Member{}, err
	}
	m := domain.Member{GuildID: inv.GuildID, UserID: userID, Role: domain.RoleMember, JoinedAt: now, UpdatedAt: now}
	if err := s.d.Members.AddMember(ctx, m); err != nil {
		return domain.Member{}, err
	}
	return m, nil
}

func (s *Service) memberAdded(m domain.Member, u domain.User) events.Event {
	presence := domain.Presence{UserID: u.ID, Status: domain.PresenceOffline}
	if ps := s.d.Presence.Presences([]domain.ID{u.ID}); len(ps) == 1 {
		presence = ps[0]
	}
	return events.Event{
		Topics:  []events.Topic{events.GuildTopic(m.GuildID)},
		Payload: events.MemberAdded{Member: m, User: u, Presence: presence},
	}
}

func (s *Service) usableInvite(ctx context.Context, code string) (domain.Invite, error) {
	notFound := domain.NotFound(domain.ReasonInviteNotFound)
	if !domain.IsGuildInviteCode(code) {
		return domain.Invite{}, notFound
	}
	inv, err := s.d.Invites.InviteByCode(ctx, code)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Invite{}, notFound
	}
	if err != nil {
		return domain.Invite{}, err
	}
	if !inv.Usable(s.d.Clock.Now()) {
		return domain.Invite{}, notFound
	}
	return inv, nil
}

func (s *Service) notBanned(ctx context.Context, guildID, userID domain.ID) error {
	_, err := s.d.Bans.BanByID(ctx, guildID, userID)
	switch {
	case err == nil:
		return domain.Forbidden(domain.ReasonBanned)
	case errors.Is(err, domain.ErrNotFound):
		return nil
	default:
		return err
	}
}
