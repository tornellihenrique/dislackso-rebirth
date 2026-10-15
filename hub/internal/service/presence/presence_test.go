package presence

import (
	"log/slog"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
)

func TestTracker(t *testing.T) {
	ctx := t.Context()
	store := fakes.NewStore()
	rec := fakes.NewEvents(store)
	_ = store.AddMember(ctx, domain.Member{GuildID: 7, UserID: 1})
	tr := New(store, rec, slog.New(slog.DiscardHandler))
	user := domain.User{ID: 1, StatusPref: domain.StatusOnline, CustomStatus: "shipping"}

	statuses := func() []domain.PresenceStatus {
		var out []domain.PresenceStatus
		for _, e := range rec.Take() {
			p := e.Payload.(events.PresenceUpdated).Presence
			if !cmp.Equal(e.Topics, []events.Topic{"user:1", "guild:7"}) {
				t.Errorf("topics = %v", e.Topics)
			}
			out = append(out, p.Status)
		}
		return out
	}

	tr.Connect(ctx, user, "a")
	tr.Connect(ctx, user, "b")
	if got := statuses(); !cmp.Equal(got, []domain.PresenceStatus{domain.PresenceOnline}) {
		t.Errorf("connect: %v", got)
	}
	if p := tr.Presences([]domain.ID{1, 2}); p[0].Status != domain.PresenceOnline || p[0].CustomStatus != "shipping" || p[1].Status != domain.PresenceOffline {
		t.Errorf("Presences = %+v", p)
	}

	tr.SetIdle(ctx, 1, "a", true)
	if got := statuses(); len(got) != 0 {
		t.Errorf("one idle connection of two changes nothing: %v", got)
	}
	tr.SetIdle(ctx, 1, "b", true)
	tr.SetIdle(ctx, 1, "b", false)
	tr.SetPreference(ctx, 1, domain.StatusDND, "")
	tr.SetPreference(ctx, 1, domain.StatusInvisible, "")
	tr.SetPreference(ctx, 1, domain.StatusOnline, "")
	want := []domain.PresenceStatus{domain.PresenceIdle, domain.PresenceOnline, domain.PresenceDND, domain.PresenceOffline, domain.PresenceOnline}
	if got := statuses(); !cmp.Equal(got, want) {
		t.Errorf("transitions = %v, want %v", got, want)
	}

	tr.Disconnect(ctx, 1, "a")
	tr.Disconnect(ctx, 1, "b")
	tr.SetIdle(ctx, 1, "b", true)
	if got := statuses(); !cmp.Equal(got, []domain.PresenceStatus{domain.PresenceOffline}) {
		t.Errorf("disconnect: %v", got)
	}
	if p := tr.Presences([]domain.ID{1}); p[0].Status != domain.PresenceOffline {
		t.Errorf("after disconnect: %+v", p)
	}
	tr.SetPreference(ctx, 1, domain.StatusDND, "")
	if got := statuses(); len(got) != 0 {
		t.Errorf("preference of an offline user published: %v", got)
	}
}
