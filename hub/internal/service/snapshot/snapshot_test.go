package snapshot

import (
	"context"
	"errors"
	"testing"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
)

type settings struct{}

func (settings) Get(context.Context) (domain.InstanceSettings, error) {
	st := domain.DefaultInstanceSettings()
	st.Name = "Team hub"
	return st, nil
}

func TestReady(t *testing.T) {
	ctx := t.Context()
	store := fakes.NewStore()
	svc := New(Deps{
		Users: store, Guilds: store, Members: store, Channels: store, Settings: settings{},
		Presence:     &fakes.Presence{Status: map[domain.ID]domain.PresenceStatus{2: domain.PresenceDND}},
		Verification: fakes.Verification(true),
	})
	for _, id := range []domain.ID{1, 2} {
		_ = store.CreateUser(ctx, domain.User{ID: id, Username: "u" + id.String(), Email: id.String() + "@x.io"})
	}
	for _, g := range []domain.ID{10, 11} {
		_ = store.CreateGuild(ctx, domain.Guild{ID: g, Name: "g" + g.String(), OwnerID: 1})
		_ = store.AddMember(ctx, domain.Member{GuildID: g, UserID: 1, Role: domain.RoleOwner})
		_ = store.CreateChannel(ctx, domain.Channel{ID: g * 10, GuildID: g, Type: domain.ChannelText, Name: "general"})
	}
	_ = store.AddMember(ctx, domain.Member{GuildID: 10, UserID: 2, Role: domain.RoleMember})
	_ = store.CreateGuild(ctx, domain.Guild{ID: 12, Name: "not mine", OwnerID: 2})
	_ = store.AddMember(ctx, domain.Member{GuildID: 12, UserID: 2, Role: domain.RoleOwner})

	r, err := svc.Ready(ctx, domain.Actor{UserID: 1, EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Me.ID != 1 || r.Instance.Name != "Team hub" || len(r.Guilds) != 2 {
		t.Fatalf("ready = %+v", r)
	}
	first := r.Guilds[0]
	if first.Guild.ID != 10 || len(first.Members) != 2 || len(first.Users) != 2 || len(first.Channels) != 1 {
		t.Errorf("first guild = %+v", first)
	}
	if first.Presences[1].UserID != 2 || first.Presences[1].Status != domain.PresenceDND {
		t.Errorf("presences = %+v", first.Presences)
	}

	ms, _ := svc.Memberships(ctx, 1)
	if len(ms) != 2 {
		t.Errorf("memberships = %+v", ms)
	}
	if _, err := svc.Ready(ctx, domain.Actor{UserID: 1}); !errors.Is(err, domain.ErrFailedPrecondition) {
		t.Errorf("unverified: %v", err)
	}
}
