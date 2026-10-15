// Package snapshot builds the state a client starts from when its gateway
// session begins (the Ready payload).
package snapshot

import (
	"context"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/internal/access"
)

type SettingsReader interface {
	Get(ctx context.Context) (domain.InstanceSettings, error)
}

type Deps struct {
	Users        ports.UserRepo
	Guilds       ports.GuildRepo
	Members      ports.MemberRepo
	Channels     ports.ChannelRepo
	Settings     SettingsReader
	Presence     access.Presences
	Voice        access.VoiceStates
	Verification access.Verification
}

type Service struct {
	d      Deps
	access access.Guilds
}

func New(d Deps) *Service {
	return &Service{d: d, access: access.Guilds{
		Guilds: d.Guilds, Members: d.Members, Channels: d.Channels, Users: d.Users,
		Presence: d.Presence, Verification: d.Verification, Voice: d.Voice,
	}}
}

type Ready struct {
	Me       domain.User
	Instance domain.InstanceSettings
	Guilds   []domain.GuildState
}

// Memberships lists the user's guilds and roles, so the gateway can
// subscribe before reading the snapshot and miss nothing in between.
func (s *Service) Memberships(ctx context.Context, userID domain.ID) ([]domain.Member, error) {
	return s.d.Members.UserMemberships(ctx, userID)
}

func (s *Service) Ready(ctx context.Context, actor domain.Actor) (Ready, error) {
	if err := s.access.Verified(ctx, actor); err != nil {
		return Ready{}, err
	}
	me, err := s.d.Users.UserByID(ctx, actor.UserID)
	if err != nil {
		return Ready{}, err
	}
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return Ready{}, err
	}
	guilds, err := s.d.Guilds.UserGuilds(ctx, actor.UserID)
	if err != nil {
		return Ready{}, err
	}
	r := Ready{Me: me, Instance: st, Guilds: make([]domain.GuildState, 0, len(guilds))}
	for _, g := range guilds {
		state, err := s.access.State(ctx, g)
		if err != nil {
			return Ready{}, err
		}
		r.Guilds = append(r.Guilds, state)
	}
	return r, nil
}
