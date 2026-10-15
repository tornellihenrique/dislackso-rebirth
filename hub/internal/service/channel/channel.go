// Package channel manages a guild's channels and categories.
package channel

import (
	"context"
	"strconv"

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

func (s *Service) Create(ctx context.Context, actor domain.Actor, guildID domain.ID, t domain.ChannelType, name string, parentID domain.ID) (domain.Channel, error) {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return domain.Channel{}, err
	}
	if err := s.policy.Guild(m, domain.ActManageChannels); err != nil {
		return domain.Channel{}, err
	}
	if !t.Valid() {
		return domain.Channel{}, domain.Invalid(domain.FieldViolation{Field: "type", Reason: domain.ReasonFieldInvalid})
	}
	name = domain.NormalizeChannelName(t, name)
	if v := domain.ValidateChannelName("name", name); v != nil {
		return domain.Channel{}, domain.Invalid(*v)
	}
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return domain.Channel{}, err
	}

	var c domain.Channel
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		all, err := s.d.Channels.GuildChannels(ctx, guildID)
		if err != nil {
			return err
		}
		if len(all) >= st.Limits.MaxChannelsPerGuild {
			return domain.Precondition(domain.ReasonChannelLimitReached)
		}
		if err := validParent(all, t, parentID); err != nil {
			return err
		}
		now := s.d.Clock.Now()
		c = domain.Channel{
			ID: s.d.IDs.NewID(), GuildID: guildID, ParentID: parentID, Type: t, Name: name,
			Position: nextPosition(all, t, parentID), CreatedAt: now, UpdatedAt: now,
		}
		if t == domain.ChannelVoice {
			c.UserLimit, c.DrawingAllowed = st.Limits.DefaultVoiceUserLimit, true
		}
		if err := s.d.Channels.CreateChannel(ctx, c); err != nil {
			return err
		}
		if err := s.audit(ctx, actor, c, domain.AuditChannelCreate, map[string]string{"name": c.Name, "type": string(c.Type)}); err != nil {
			return err
		}
		s.publish(ctx, guildID, events.ChannelCreated{Channel: c})
		return nil
	})
	return c, err
}

// validParent accepts the top level, or a category of the guild for
// non-category channels.
func validParent(all []domain.Channel, t domain.ChannelType, parentID domain.ID) error {
	if parentID == 0 {
		return nil
	}
	invalid := domain.Invalid(domain.FieldViolation{Field: "parent_id", Reason: domain.ReasonFieldInvalid})
	if t == domain.ChannelCategory {
		return invalid
	}
	for _, c := range all {
		if c.ID == parentID {
			if c.Type != domain.ChannelCategory {
				return invalid
			}
			return nil
		}
	}
	return invalid
}

// siblings reports whether c is ordered together with channels of type t
// under parentID: categories among categories, others within their parent.
func siblings(c domain.Channel, t domain.ChannelType, parentID domain.ID) bool {
	if t == domain.ChannelCategory {
		return c.Type == domain.ChannelCategory
	}
	return c.Type != domain.ChannelCategory && c.ParentID == parentID
}

func nextPosition(all []domain.Channel, t domain.ChannelType, parentID domain.ID) int {
	next := 0
	for _, c := range all {
		if siblings(c, t, parentID) && c.Position >= next {
			next = c.Position + 1
		}
	}
	return next
}

// Patch changes the fields that are set. Voice-only fields are refused for
// other channel types, and categories have no topic.
type Patch struct {
	Name            *string
	Topic           *string
	UserLimit       *int
	DrawingAllowed  *bool
	MaxShareQuality *string
}

func (s *Service) Update(ctx context.Context, actor domain.Actor, channelID domain.ID, p Patch) (domain.Channel, error) {
	c, m, err := s.access.Channel(ctx, actor, channelID)
	if err != nil {
		return domain.Channel{}, err
	}
	if err := s.policy.Guild(m, domain.ActManageChannels); err != nil {
		return domain.Channel{}, err
	}
	data, err := apply(&c, p)
	if err != nil {
		return domain.Channel{}, err
	}
	if len(data) == 0 {
		return c, nil
	}
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		c.UpdatedAt = s.d.Clock.Now()
		if err := s.d.Channels.UpdateChannel(ctx, c); err != nil {
			return err
		}
		if err := s.audit(ctx, actor, c, domain.AuditChannelUpdate, data); err != nil {
			return err
		}
		s.publish(ctx, c.GuildID, events.ChannelUpdated{Channel: c})
		return nil
	})
	return c, err
}

// apply validates p, writes it to c and returns the changes for the audit log.
func apply(c *domain.Channel, p Patch) (map[string]string, error) {
	var checks []*domain.FieldViolation
	voiceOnly := func(field string, set bool) {
		if set && c.Type != domain.ChannelVoice {
			checks = append(checks, &domain.FieldViolation{Field: field, Reason: domain.ReasonFieldInvalid})
		}
	}
	voiceOnly("user_limit", p.UserLimit != nil)
	voiceOnly("drawing_allowed", p.DrawingAllowed != nil)
	voiceOnly("max_share_quality", p.MaxShareQuality != nil)

	var name, topic string
	if p.Name != nil {
		name = domain.NormalizeChannelName(c.Type, *p.Name)
		checks = append(checks, domain.ValidateChannelName("name", name))
	}
	if p.Topic != nil {
		topic = *p.Topic
		if c.Type == domain.ChannelCategory {
			checks = append(checks, &domain.FieldViolation{Field: "topic", Reason: domain.ReasonFieldInvalid})
		}
		checks = append(checks, domain.ValidateChannelTopic("topic", topic))
	}
	if p.UserLimit != nil {
		checks = append(checks, domain.ValidateUserLimit("user_limit", *p.UserLimit))
	}
	if p.MaxShareQuality != nil {
		checks = append(checks, domain.ValidateSharePreset("max_share_quality", *p.MaxShareQuality))
	}
	if err := domain.Violations(checks...); err != nil {
		return nil, err
	}

	data := map[string]string{}
	if p.Name != nil && name != c.Name {
		data["old_name"], data["name"] = c.Name, name
		c.Name = name
	}
	if p.Topic != nil && topic != c.Topic {
		data["topic"] = topic
		c.Topic = topic
	}
	if p.UserLimit != nil && *p.UserLimit != c.UserLimit {
		data["user_limit"] = strconv.Itoa(*p.UserLimit)
		c.UserLimit = *p.UserLimit
	}
	if p.DrawingAllowed != nil && *p.DrawingAllowed != c.DrawingAllowed {
		data["drawing_allowed"] = strconv.FormatBool(*p.DrawingAllowed)
		c.DrawingAllowed = *p.DrawingAllowed
	}
	if p.MaxShareQuality != nil && *p.MaxShareQuality != c.MaxShareQuality {
		data["max_share_quality"] = *p.MaxShareQuality
		c.MaxShareQuality = *p.MaxShareQuality
	}
	return data, nil
}

// Delete removes a channel. A category's channels move to the end of the top level.
func (s *Service) Delete(ctx context.Context, actor domain.Actor, channelID domain.ID) error {
	c, m, err := s.access.Channel(ctx, actor, channelID)
	if err != nil {
		return err
	}
	if err := s.policy.Guild(m, domain.ActManageChannels); err != nil {
		return err
	}
	return s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		var moved []domain.Channel
		if c.Type == domain.ChannelCategory {
			all, err := s.d.Channels.GuildChannels(ctx, c.GuildID)
			if err != nil {
				return err
			}
			now := s.d.Clock.Now()
			next := nextPosition(all, domain.ChannelText, 0)
			for _, child := range all {
				if child.ParentID != c.ID {
					continue
				}
				child.ParentID, child.Position, child.UpdatedAt = 0, next, now
				next++
				if err := s.d.Channels.UpdateChannel(ctx, child); err != nil {
					return err
				}
				moved = append(moved, child)
			}
		}
		if err := s.d.Channels.DeleteChannel(ctx, c.ID); err != nil {
			return err
		}
		if err := s.audit(ctx, actor, c, domain.AuditChannelDelete, map[string]string{"name": c.Name, "type": string(c.Type)}); err != nil {
			return err
		}
		s.publish(ctx, c.GuildID, events.ChannelDeleted{GuildID: c.GuildID, ChannelID: c.ID})
		if len(moved) > 0 {
			s.publish(ctx, c.GuildID, events.ChannelsReordered{GuildID: c.GuildID, Channels: moved})
		}
		return nil
	})
}

// Placement puts a channel under a category, or at the top level when
// ParentID is zero.
type Placement struct {
	ChannelID domain.ID
	ParentID  domain.ID
}

// Reorder applies a full ordering of the guild's channels: every channel
// exactly once, in display order.
func (s *Service) Reorder(ctx context.Context, actor domain.Actor, guildID domain.ID, placements []Placement) ([]domain.Channel, error) {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return nil, err
	}
	if err := s.policy.Guild(m, domain.ActManageChannels); err != nil {
		return nil, err
	}
	var result []domain.Channel
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		all, err := s.d.Channels.GuildChannels(ctx, guildID)
		if err != nil {
			return err
		}
		changed, ordered, err := reorder(all, placements)
		if err != nil {
			return err
		}
		result = ordered
		if len(changed) == 0 {
			return nil
		}
		now := s.d.Clock.Now()
		for i := range changed {
			changed[i].UpdatedAt = now
			if err := s.d.Channels.UpdateChannel(ctx, changed[i]); err != nil {
				return err
			}
		}
		for i := range result {
			for _, c := range changed {
				if result[i].ID == c.ID {
					result[i] = c
				}
			}
		}
		if err := s.auditor.Record(ctx, access.Entry{
			GuildID: guildID, ActorID: actor.UserID, Action: domain.AuditChannelsReorder,
			TargetType: domain.AuditTargetGuild, TargetID: guildID.String(),
			Data: map[string]string{"channels": strconv.Itoa(len(changed))},
		}); err != nil {
			return err
		}
		s.publish(ctx, guildID, events.ChannelsReordered{GuildID: guildID, Channels: changed})
		return nil
	})
	return result, err
}

// reorder computes new parents and positions; it returns the channels that
// changed and every channel in its new state.
func reorder(all []domain.Channel, placements []Placement) (changed, ordered []domain.Channel, err error) {
	invalid := domain.Invalid(domain.FieldViolation{Field: "placements", Reason: domain.ReasonFieldInvalid})
	byID := make(map[domain.ID]domain.Channel, len(all))
	for _, c := range all {
		byID[c.ID] = c
	}
	if len(placements) != len(all) {
		return nil, nil, invalid
	}
	seen := map[domain.ID]bool{}
	next := map[domain.ID]int{}
	categories := 0
	for _, p := range placements {
		c, ok := byID[p.ChannelID]
		if !ok || seen[p.ChannelID] {
			return nil, nil, invalid
		}
		seen[p.ChannelID] = true
		if p.ParentID != 0 {
			parent, ok := byID[p.ParentID]
			if !ok || parent.Type != domain.ChannelCategory || c.Type == domain.ChannelCategory {
				return nil, nil, invalid
			}
		}
		updated := c
		updated.ParentID = p.ParentID
		if c.Type == domain.ChannelCategory {
			updated.Position = categories
			categories++
		} else {
			updated.Position = next[p.ParentID]
			next[p.ParentID]++
		}
		if updated.ParentID != c.ParentID || updated.Position != c.Position {
			changed = append(changed, updated)
		}
		ordered = append(ordered, updated)
	}
	return changed, ordered, nil
}

func (s *Service) audit(ctx context.Context, actor domain.Actor, c domain.Channel, action domain.AuditAction, data map[string]string) error {
	return s.auditor.Record(ctx, access.Entry{
		GuildID: c.GuildID, ActorID: actor.UserID, Action: action,
		TargetType: domain.AuditTargetChannel, TargetID: c.ID.String(), Data: data,
	})
}

func (s *Service) publish(ctx context.Context, guildID domain.ID, p events.Payload) {
	s.d.Events.Publish(ctx, events.Event{Topics: []events.Topic{events.GuildTopic(guildID)}, Payload: p})
}
