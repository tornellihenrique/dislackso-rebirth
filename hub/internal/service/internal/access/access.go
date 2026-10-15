// Package access holds what the guild-related services share: membership
// checks that keep guilds private, guild state loading and audit entries.
package access

import (
	"context"
	"errors"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

// Verification tells whether accounts must confirm their email first.
type Verification interface {
	VerificationRequired(ctx context.Context) bool
}

// Presences reports what others currently see for each user.
type Presences interface {
	Presences(userIDs []domain.ID) []domain.Presence
}

// VoiceStates reports who is in a guild's voice channels.
type VoiceStates interface {
	GuildVoiceStates(guildID domain.ID) []domain.VoiceState
}

type Guilds struct {
	Guilds       ports.GuildRepo
	Members      ports.MemberRepo
	Channels     ports.ChannelRepo
	Users        ports.UserRepo
	Presence     Presences
	Verification Verification
	// Voice is optional; without it guild states carry no voice states.
	Voice VoiceStates
}

// Verified refuses accounts that still have to confirm their email.
func (g Guilds) Verified(ctx context.Context, actor domain.Actor) error {
	if !actor.EmailVerified && g.Verification.VerificationRequired(ctx) {
		return domain.Precondition(domain.ReasonEmailNotVerified)
	}
	return nil
}

// Member returns the caller's membership. A guild the caller is not in
// answers NotFound, so its existence stays private.
func (g Guilds) Member(ctx context.Context, actor domain.Actor, guildID domain.ID) (domain.Member, error) {
	if err := g.Verified(ctx, actor); err != nil {
		return domain.Member{}, err
	}
	m, err := g.Members.MemberByID(ctx, guildID, actor.UserID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Member{}, domain.NotFound(domain.ReasonGuildNotFound)
	}
	return m, err
}

// Target returns another member of the guild.
func (g Guilds) Target(ctx context.Context, guildID, userID domain.ID) (domain.Member, error) {
	m, err := g.Members.MemberByID(ctx, guildID, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Member{}, domain.NotFound(domain.ReasonMemberNotFound)
	}
	return m, err
}

// Channel returns a channel the caller can see, with the caller's membership
// in its guild.
func (g Guilds) Channel(ctx context.Context, actor domain.Actor, id domain.ID) (domain.Channel, domain.Member, error) {
	notFound := domain.NotFound(domain.ReasonChannelNotFound)
	c, err := g.Channels.ChannelByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && c.GuildID == 0) {
		return domain.Channel{}, domain.Member{}, notFound
	}
	if err != nil {
		return domain.Channel{}, domain.Member{}, err
	}
	m, err := g.Member(ctx, actor, c.GuildID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Channel{}, domain.Member{}, notFound
	}
	return c, m, err
}

// State loads everything a member needs to show the guild.
func (g Guilds) State(ctx context.Context, guild domain.Guild) (domain.GuildState, error) {
	channels, err := g.Channels.GuildChannels(ctx, guild.ID)
	if err != nil {
		return domain.GuildState{}, err
	}
	members, err := g.Members.GuildMembers(ctx, guild.ID)
	if err != nil {
		return domain.GuildState{}, err
	}
	users, err := g.Users.GuildUsers(ctx, guild.ID)
	if err != nil {
		return domain.GuildState{}, err
	}
	ids := make([]domain.ID, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	state := domain.GuildState{
		Guild: guild, Channels: channels, Members: members, Users: users,
		Presences: g.Presence.Presences(ids),
	}
	if g.Voice != nil {
		state.VoiceStates = g.Voice.GuildVoiceStates(guild.ID)
	}
	return state, nil
}

// OwnFile checks that id is a file the caller uploaded for purpose; field
// names the request field for the error.
func OwnFile(ctx context.Context, files ports.FileRepo, actor domain.Actor, id domain.ID, purpose domain.FilePurpose, field string) error {
	f, err := files.FileByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && (f.OwnerID != actor.UserID || f.Purpose != purpose)) {
		return domain.Invalid(domain.FieldViolation{Field: field, Reason: domain.ReasonFieldInvalid})
	}
	return err
}

// Auditor appends audit entries and tells the guild's admins.
type Auditor struct {
	Repo   ports.AuditRepo
	IDs    ports.IDGen
	Clock  ports.Clock
	Events ports.EventPublisher
}

type Entry struct {
	GuildID    domain.ID
	ActorID    domain.ID
	Action     domain.AuditAction
	TargetType domain.AuditTarget
	TargetID   string
	Data       map[string]string
}

func (a Auditor) Record(ctx context.Context, e Entry) error {
	entry := domain.AuditEntry{
		ID:         a.IDs.NewID(),
		GuildID:    e.GuildID,
		ActorID:    e.ActorID,
		Action:     e.Action,
		TargetType: e.TargetType,
		TargetID:   e.TargetID,
		Data:       e.Data,
		CreatedAt:  a.Clock.Now(),
	}
	if err := a.Repo.AppendAudit(ctx, entry); err != nil {
		return err
	}
	a.Events.Publish(ctx, events.Event{
		Topics:  []events.Topic{events.GuildAdminsTopic(e.GuildID)},
		Payload: events.AuditAppended{Entry: entry},
	})
	return nil
}

// GuildTopics addresses every guild the user is in, plus the user's own
// sessions: everyone who can see the user's profile.
func GuildTopics(ctx context.Context, members ports.MemberRepo, userID domain.ID) ([]events.Topic, error) {
	ms, err := members.UserMemberships(ctx, userID)
	if err != nil {
		return nil, err
	}
	topics := make([]events.Topic, 0, len(ms)+1)
	topics = append(topics, events.UserTopic(userID))
	for _, m := range ms {
		topics = append(topics, events.GuildTopic(m.GuildID))
	}
	return topics, nil
}
