package connectivity

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/relay"
)

type relayStub struct{ ice []ports.IceServer }

func (r relayStub) Usage(context.Context) (domain.TURNUsage, error) { return domain.TURNUsage{}, nil }

func (r relayStub) Apply(context.Context, domain.Actor, relay.Patch) (domain.InstanceSettings, error) {
	return domain.InstanceSettings{}, nil
}

func (r relayStub) Issue(context.Context, domain.ID, time.Duration) ([]ports.IceServer, error) {
	return r.ice, nil
}

func TestRelayTest(t *testing.T) {
	admin := domain.Actor{UserID: 1, InstanceAdmin: true}
	turn := []ports.IceServer{{URLs: []string{"turn:turn.example:3478"}, Username: "u", Credential: "c"}}
	media := &fakes.Media{}
	svc := New(Deps{Relay: relayStub{ice: turn}, Media: media, Tokens: &fakes.Tokens{}})

	if _, err := svc.RelayTest(t.Context(), domain.Actor{UserID: 2}); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("member err = %v, want forbidden", err)
	}
	got, err := svc.RelayTest(t.Context(), admin)
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "/lk" || len(got.ICE) != 1 || got.Token == "" {
		t.Errorf("relay test = %+v", got)
	}
	room, _, _ := strings.Cut(got.Token, "|")
	if !strings.HasPrefix(room, relayTestRoomPrefix) {
		t.Fatalf("token for room %q, want a diagnostics room", room)
	}
	// Voice owns vc_ rooms; the test room must stay out of voice state.
	if _, ok := domain.ParseVoiceRoom(room); ok {
		t.Errorf("room %q parses as a voice room", room)
	}

	noRelay := New(Deps{Relay: relayStub{}, Media: media, Tokens: &fakes.Tokens{}})
	if _, err := noRelay.RelayTest(t.Context(), admin); !errors.Is(err, domain.ErrFailedPrecondition) {
		t.Errorf("without a relay err = %v, want precondition", err)
	}
}
