// Package guild creates and manages guilds and their members' roles.
package guild

import (
	"context"
	"strings"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/internal/access"
)

type SettingsReader interface {
	Get(ctx context.Context) (domain.InstanceSettings, error)
}

type Deps struct {
	Guilds       ports.GuildRepo
	Members      ports.MemberRepo
	Channels     ports.ChannelRepo
	Users        ports.UserRepo
	Files        ports.FileRepo
	Audit        ports.AuditRepo
	Tx           ports.TxRunner
	IDs          ports.IDGen
	Clock        ports.Clock
	Events       ports.EventPublisher
	Settings     SettingsReader
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

// defaultChannels are the channels of a new guild, named in the creator's language.
var defaultChannels = map[string]struct{ textCategory, text, voiceCategory, voice string }{
	"en":    {"Text Channels", "general", "Voice Channels", "General"},
	"pt-BR": {"Canais de texto", "geral", "Canais de voz", "Geral"},
}

// Create makes a guild owned by the caller, with a text and a voice channel.
func (s *Service) Create(ctx context.Context, actor domain.Actor, locale, name string, iconFileID domain.ID) (domain.GuildState, error) {
	if err := s.access.Verified(ctx, actor); err != nil {
		return domain.GuildState{}, err
	}
	name = domain.NormalizeGuildName(name)
	if v := domain.ValidateGuildName("name", name); v != nil {
		return domain.GuildState{}, domain.Invalid(*v)
	}
	if iconFileID != 0 {
		if err := access.OwnFile(ctx, s.d.Files, actor, iconFileID, domain.FileGuildIcon, "icon_file_id"); err != nil {
			return domain.GuildState{}, err
		}
	}
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return domain.GuildState{}, err
	}
	names, ok := defaultChannels[locale]
	if !ok {
		names = defaultChannels["en"]
	}

	var state domain.GuildState
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		owned, err := s.d.Guilds.CountGuildsOwnedBy(ctx, actor.UserID)
		if err != nil {
			return err
		}
		if owned >= st.Limits.MaxGuildsOwnedPerUser {
			return domain.Precondition(domain.ReasonGuildLimitReached)
		}
		now := s.d.Clock.Now()
		g := domain.Guild{ID: s.d.IDs.NewID(), Name: name, IconFileID: iconFileID, OwnerID: actor.UserID, CreatedAt: now, UpdatedAt: now}
		if err := s.d.Guilds.CreateGuild(ctx, g); err != nil {
			return err
		}
		owner := domain.Member{GuildID: g.ID, UserID: actor.UserID, Role: domain.RoleOwner, JoinedAt: now, UpdatedAt: now}
		if err := s.d.Members.AddMember(ctx, owner); err != nil {
			return err
		}
		textCat := s.newChannel(g.ID, 0, domain.ChannelCategory, names.textCategory, 0)
		voiceCat := s.newChannel(g.ID, 0, domain.ChannelCategory, names.voiceCategory, 1)
		text := s.newChannel(g.ID, textCat.ID, domain.ChannelText, names.text, 0)
		voice := s.newChannel(g.ID, voiceCat.ID, domain.ChannelVoice, names.voice, 0)
		voice.UserLimit, voice.DrawingAllowed = st.Limits.DefaultVoiceUserLimit, true
		for _, c := range []domain.Channel{textCat, voiceCat, text, voice} {
			if err := s.d.Channels.CreateChannel(ctx, c); err != nil {
				return err
			}
		}
		if state, err = s.access.State(ctx, g); err != nil {
			return err
		}
		s.d.Events.Publish(ctx, events.Event{
			Topics:  []events.Topic{events.UserTopic(actor.UserID)},
			Payload: events.GuildCreated{UserID: actor.UserID, State: state},
		})
		return nil
	})
	return state, err
}

func (s *Service) newChannel(guildID, parentID domain.ID, t domain.ChannelType, name string, pos int) domain.Channel {
	now := s.d.Clock.Now()
	return domain.Channel{
		ID: s.d.IDs.NewID(), GuildID: guildID, ParentID: parentID, Type: t,
		Name: name, Position: pos, CreatedAt: now, UpdatedAt: now,
	}
}

// Patch changes the fields that are set; a zero IconFileID removes the icon.
type Patch struct {
	Name       *string
	IconFileID *domain.ID
}

func (s *Service) Update(ctx context.Context, actor domain.Actor, guildID domain.ID, p Patch) (domain.Guild, error) {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return domain.Guild{}, err
	}
	if err := s.policy.Guild(m, domain.ActUpdateGuild); err != nil {
		return domain.Guild{}, err
	}
	var name string
	if p.Name != nil {
		name = domain.NormalizeGuildName(*p.Name)
		if v := domain.ValidateGuildName("name", name); v != nil {
			return domain.Guild{}, domain.Invalid(*v)
		}
	}
	if p.IconFileID != nil && *p.IconFileID != 0 {
		if err := access.OwnFile(ctx, s.d.Files, actor, *p.IconFileID, domain.FileGuildIcon, "icon_file_id"); err != nil {
			return domain.Guild{}, err
		}
	}

	var g domain.Guild
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if g, err = s.d.Guilds.GuildByID(ctx, guildID); err != nil {
			return err
		}
		data := map[string]string{}
		if p.Name != nil && name != g.Name {
			data["old_name"], data["name"] = g.Name, name
			g.Name = name
		}
		if p.IconFileID != nil && *p.IconFileID != g.IconFileID {
			data["icon"] = "changed"
			if *p.IconFileID == 0 {
				data["icon"] = "removed"
			}
			g.IconFileID = *p.IconFileID
		}
		if len(data) == 0 {
			return nil
		}
		g.UpdatedAt = s.d.Clock.Now()
		if err := s.d.Guilds.UpdateGuild(ctx, g); err != nil {
			return err
		}
		if err := s.auditor.Record(ctx, access.Entry{
			GuildID: g.ID, ActorID: actor.UserID, Action: domain.AuditGuildUpdate,
			TargetType: domain.AuditTargetGuild, TargetID: g.ID.String(), Data: data,
		}); err != nil {
			return err
		}
		s.d.Events.Publish(ctx, events.Event{
			Topics:  []events.Topic{events.GuildTopic(g.ID)},
			Payload: events.GuildUpdated{Guild: g},
		})
		return nil
	})
	return g, err
}

// Delete removes the guild with everything in it. The owner types its name
// to confirm.
func (s *Service) Delete(ctx context.Context, actor domain.Actor, guildID domain.ID, confirmName string) error {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return err
	}
	if err := s.policy.Guild(m, domain.ActDeleteGuild); err != nil {
		return err
	}
	return s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		g, err := s.d.Guilds.GuildByID(ctx, guildID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(confirmName) != g.Name {
			return domain.Invalid(domain.FieldViolation{Field: "confirm_name", Reason: domain.ReasonFieldIncorrect})
		}
		if err := s.d.Guilds.DeleteGuild(ctx, guildID); err != nil {
			return err
		}
		s.d.Events.Publish(ctx, events.Event{
			Topics:  []events.Topic{events.GuildTopic(guildID)},
			Payload: events.GuildDeleted{GuildID: guildID, Reason: events.ReasonGuildDeleted},
		})
		return nil
	})
}

func (s *Service) Leave(ctx context.Context, actor domain.Actor, guildID domain.ID) error {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return err
	}
	if err := s.policy.LeaveGuild(m); err != nil {
		return err
	}
	return s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Members.RemoveMember(ctx, guildID, actor.UserID); err != nil {
			return err
		}
		RemovedMember(ctx, s.d.Events, guildID, actor.UserID, events.ReasonLeft)
		return nil
	})
}

// RemovedMember tells a user who left or was removed that the guild is gone
// for them, and tells the remaining members.
func RemovedMember(ctx context.Context, pub ports.EventPublisher, guildID, userID domain.ID, reason events.GuildDeleteReason) {
	pub.Publish(ctx,
		events.Event{
			Topics:  []events.Topic{events.UserTopic(userID)},
			Payload: events.GuildDeleted{GuildID: guildID, UserID: userID, Reason: reason},
		},
		events.Event{
			Topics:  []events.Topic{events.GuildTopic(guildID)},
			Payload: events.MemberRemoved{GuildID: guildID, UserID: userID},
		},
	)
}

// TransferOwnership makes another member the owner; the previous owner
// becomes an admin.
func (s *Service) TransferOwnership(ctx context.Context, actor domain.Actor, guildID, targetID domain.ID) (domain.Guild, error) {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return domain.Guild{}, err
	}
	target, err := s.access.Target(ctx, guildID, targetID)
	if err != nil {
		return domain.Guild{}, err
	}
	if err := s.policy.TransferOwnership(m, target); err != nil {
		return domain.Guild{}, err
	}
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return domain.Guild{}, err
	}
	var g domain.Guild
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		owned, err := s.d.Guilds.CountGuildsOwnedBy(ctx, targetID)
		if err != nil {
			return err
		}
		if owned >= st.Limits.MaxGuildsOwnedPerUser {
			return domain.Precondition(domain.ReasonGuildLimitReached)
		}
		if g, err = s.d.Guilds.GuildByID(ctx, guildID); err != nil {
			return err
		}
		now := s.d.Clock.Now()
		g.OwnerID, g.UpdatedAt = targetID, now
		if err := s.d.Guilds.UpdateGuild(ctx, g); err != nil {
			return err
		}
		m.Role, m.UpdatedAt = domain.RoleAdmin, now
		target.Role, target.UpdatedAt = domain.RoleOwner, now
		for _, mm := range []domain.Member{target, m} {
			if err := s.d.Members.UpdateMemberRole(ctx, guildID, mm.UserID, mm.Role, now); err != nil {
				return err
			}
		}
		if err := s.auditor.Record(ctx, access.Entry{
			GuildID: guildID, ActorID: actor.UserID, Action: domain.AuditOwnershipTransfer,
			TargetType: domain.AuditTargetUser, TargetID: targetID.String(),
		}); err != nil {
			return err
		}
		topic := []events.Topic{events.GuildTopic(guildID)}
		s.d.Events.Publish(ctx,
			events.Event{Topics: topic, Payload: events.GuildUpdated{Guild: g}},
			events.Event{Topics: topic, Payload: events.MemberUpdated{Member: target}},
			events.Event{Topics: topic, Payload: events.MemberUpdated{Member: m}},
		)
		return nil
	})
	return g, err
}

// SetMemberRole makes a member an admin or back.
func (s *Service) SetMemberRole(ctx context.Context, actor domain.Actor, guildID, targetID domain.ID, role domain.GuildRole) (domain.Member, error) {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return domain.Member{}, err
	}
	target, err := s.access.Target(ctx, guildID, targetID)
	if err != nil {
		return domain.Member{}, err
	}
	if err := s.policy.SetMemberRole(m, target, role); err != nil {
		return domain.Member{}, err
	}
	if target.Role == role {
		return target, nil
	}
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		now := s.d.Clock.Now()
		if err := s.d.Members.UpdateMemberRole(ctx, guildID, targetID, role, now); err != nil {
			return err
		}
		target.Role, target.UpdatedAt = role, now
		if err := s.auditor.Record(ctx, access.Entry{
			GuildID: guildID, ActorID: actor.UserID, Action: domain.AuditMemberRoleUpdate,
			TargetType: domain.AuditTargetUser, TargetID: targetID.String(),
			Data: map[string]string{"role": string(role)},
		}); err != nil {
			return err
		}
		s.d.Events.Publish(ctx, events.Event{
			Topics:  []events.Topic{events.GuildTopic(guildID)},
			Payload: events.MemberUpdated{Member: target},
		})
		return nil
	})
	return target, err
}

type MemberList struct {
	Members   []domain.Member
	Users     []domain.User
	Presences []domain.Presence
}

func (s *Service) Members(ctx context.Context, actor domain.Actor, guildID domain.ID) (MemberList, error) {
	if _, err := s.access.Member(ctx, actor, guildID); err != nil {
		return MemberList{}, err
	}
	g, err := s.d.Guilds.GuildByID(ctx, guildID)
	if err != nil {
		return MemberList{}, err
	}
	state, err := s.access.State(ctx, g)
	if err != nil {
		return MemberList{}, err
	}
	return MemberList{Members: state.Members, Users: state.Users, Presences: state.Presences}, nil
}
