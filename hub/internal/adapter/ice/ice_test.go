package ice

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

func TestProviders(t *testing.T) {
	ctx := t.Context()
	if got, err := (None{}).Issue(ctx, 1, time.Hour); err != nil || got != nil {
		t.Errorf("none = %v, %v", got, err)
	}
	s := Static{URLs: []string{"turn:turn.example:3478?transport=udp", "turns:turn.example:443"}, Username: "team", Credential: "secret"}
	got, err := s.Issue(ctx, 1, time.Hour)
	want := []ports.IceServer{{URLs: s.URLs, Username: "team", Credential: "secret"}}
	if err != nil || !cmp.Equal(got, want) {
		t.Errorf("static = %v, %v", got, err)
	}
	got[0].URLs[0] = "changed"
	if s.URLs[0] == "changed" {
		t.Error("issued servers share the configured slice")
	}
	if got, _ := (Static{}).Issue(ctx, 1, time.Hour); got != nil {
		t.Errorf("unconfigured static = %v", got)
	}
}
