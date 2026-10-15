// Package moderation removes and bans guild members and reads the audit log.
package moderation

import (
	"context"
	"errors"
	"strings"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/guild"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/internal/access"
)

type Deps struct {
	Guilds       ports.GuildRepo
	Members      ports.MemberRepo
	Channels     ports.ChannelRepo
	Users        ports.UserRepo
	Bans         ports.BanRepo
	Audit        ports.AuditRepo
	Tx           ports.TxRunner
	IDs          ports.IDGen
	Clock        ports.Clock
	Events       ports.EventPublisher
	Presence     access.Presences
	Verification access.Verification
}

type Service struct {
	d       Deps
	access  access.Guilds
	auditor access.Auditor
	policy  domain.Policy
}

func New(d Deps) *Service {
	return &Service{
		d: d,
		access: access.Guilds{
			Guilds: d.Guilds, Members: d.Members, Channels: d.Channels, Users: d.Users,
			Presence: d.Presence, Verification: d.Verification,
		},
		auditor: access.Auditor{Repo: d.Audit, IDs: d.IDs, Clock: d.Clock, Events: d.Events},
	}
}

func (s *Service) Kick(ctx context.Context, actor domain.Actor, guildID, targetID domain.ID, reason string) error {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return err
	}
	target, err := s.access.Target(ctx, guildID, targetID)
	if err != nil {
		return err
	}
	if err := s.policy.TargetMember(m, target, domain.ActKickMembers); err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if v := domain.ValidateReason("reason", reason); v != nil {
		return domain.Invalid(*v)
	}
	return s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Members.RemoveMember(ctx, guildID, targetID); err != nil {
			return err
		}
		if err := s.record(ctx, actor, guildID, targetID, domain.AuditMemberKick, reason); err != nil {
			return err
		}
		guild.RemovedMember(ctx, s.d.Events, guildID, targetID, events.ReasonKicked)
		return nil
	})
}

// Ban removes the user if they are a member and keeps them from accepting
// invites. Former members can be banned too.
func (s *Service) Ban(ctx context.Context, actor domain.Actor, guildID, targetID domain.ID, reason string) error {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return err
	}
	user, err := s.d.Users.UserByID(ctx, targetID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.NotFound(domain.ReasonUserNotFound)
	}
	if err != nil {
		return err
	}
	target, err := s.d.Members.MemberByID(ctx, guildID, targetID)
	member := err == nil
	if !member && !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	if !member {
		target = domain.Member{GuildID: guildID, UserID: targetID, Role: domain.RoleMember}
	}
	if err := s.policy.TargetMember(m, target, domain.ActBanMembers); err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if v := domain.ValidateReason("reason", reason); v != nil {
		return domain.Invalid(*v)
	}
	return s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		ban := domain.Ban{GuildID: guildID, UserID: targetID, BannedBy: actor.UserID, Reason: reason, CreatedAt: s.d.Clock.Now()}
		if err := s.d.Bans.CreateBan(ctx, ban); errors.Is(err, domain.ErrConflict) {
			return domain.Conflict("user_id")
		} else if err != nil {
			return err
		}
		if member {
			if err := s.d.Members.RemoveMember(ctx, guildID, targetID); err != nil {
				return err
			}
		}
		if err := s.record(ctx, actor, guildID, targetID, domain.AuditMemberBan, reason); err != nil {
			return err
		}
		if member {
			guild.RemovedMember(ctx, s.d.Events, guildID, targetID, events.ReasonBanned)
		}
		bannedBy, err := s.optionalUser(ctx, actor.UserID)
		if err != nil {
			return err
		}
		s.d.Events.Publish(ctx, events.Event{
			Topics:  []events.Topic{events.GuildAdminsTopic(guildID)},
			Payload: events.BanAdded{Ban: domain.BanView{Ban: ban, User: user, BannedBy: bannedBy}},
		})
		return nil
	})
}

func (s *Service) Unban(ctx context.Context, actor domain.Actor, guildID, targetID domain.ID) error {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return err
	}
	if err := s.policy.Guild(m, domain.ActBanMembers); err != nil {
		return err
	}
	return s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		err := s.d.Bans.DeleteBan(ctx, guildID, targetID)
		if errors.Is(err, domain.ErrNotFound) {
			return domain.NotFound(domain.ReasonBanNotFound)
		}
		if err != nil {
			return err
		}
		if err := s.record(ctx, actor, guildID, targetID, domain.AuditMemberUnban, ""); err != nil {
			return err
		}
		s.d.Events.Publish(ctx, events.Event{
			Topics:  []events.Topic{events.GuildAdminsTopic(guildID)},
			Payload: events.BanRemoved{GuildID: guildID, UserID: targetID},
		})
		return nil
	})
}

func (s *Service) Bans(ctx context.Context, actor domain.Actor, guildID domain.ID) ([]domain.BanView, error) {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return nil, err
	}
	if err := s.policy.Guild(m, domain.ActBanMembers); err != nil {
		return nil, err
	}
	bans, err := s.d.Bans.GuildBans(ctx, guildID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.BanView, 0, len(bans))
	for _, b := range bans {
		u, err := s.optionalUser(ctx, b.UserID)
		if err != nil {
			return nil, err
		}
		if u == nil {
			continue
		}
		by, err := s.optionalUser(ctx, b.BannedBy)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.BanView{Ban: b, User: *u, BannedBy: by})
	}
	return out, nil
}

type AuditPage struct {
	Entries []domain.AuditEntry
	// Users are the actors and targeted users that still exist.
	Users []domain.User
}

// AuditLog pages backwards from before (zero for the newest).
func (s *Service) AuditLog(ctx context.Context, actor domain.Actor, guildID, before domain.ID, limit int) (AuditPage, error) {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return AuditPage{}, err
	}
	if err := s.policy.Guild(m, domain.ActViewAuditLog); err != nil {
		return AuditPage{}, err
	}
	if limit <= 0 {
		limit = domain.AuditPageDefault
	}
	limit = min(limit, domain.AuditPageMax)
	entries, err := s.d.Audit.AuditLog(ctx, guildID, before, limit)
	if err != nil {
		return AuditPage{}, err
	}
	page := AuditPage{Entries: entries}
	seen := map[domain.ID]bool{}
	add := func(id domain.ID) error {
		if id == 0 || seen[id] {
			return nil
		}
		seen[id] = true
		u, err := s.optionalUser(ctx, id)
		if u != nil {
			page.Users = append(page.Users, *u)
		}
		return err
	}
	for _, e := range entries {
		if err := add(e.ActorID); err != nil {
			return AuditPage{}, err
		}
		if e.TargetType == domain.AuditTargetUser {
			if id, err := domain.ParseID(e.TargetID, "target_id"); err == nil {
				if err := add(id); err != nil {
					return AuditPage{}, err
				}
			}
		}
	}
	return page, nil
}

func (s *Service) record(ctx context.Context, actor domain.Actor, guildID, targetID domain.ID, action domain.AuditAction, reason string) error {
	var data map[string]string
	if reason != "" {
		data = map[string]string{"reason": reason}
	}
	return s.auditor.Record(ctx, access.Entry{
		GuildID: guildID, ActorID: actor.UserID, Action: action,
		TargetType: domain.AuditTargetUser, TargetID: targetID.String(), Data: data,
	})
}

func (s *Service) optionalUser(ctx context.Context, id domain.ID) (*domain.User, error) {
	if id == 0 {
		return nil, nil
	}
	u, err := s.d.Users.UserByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
