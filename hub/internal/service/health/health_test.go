package health

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

type fakeProbe struct {
	name string
	err  error
}

func (p fakeProbe) Name() string                { return p.name }
func (p fakeProbe) Check(context.Context) error { return p.err }

func TestCheck(t *testing.T) {
	started := time.UnixMilli(1_000)
	later := time.UnixMilli(5_000)
	errDB := errors.New("database locked")

	tests := []struct {
		name   string
		probes []Probe
		want   Report
	}{
		{
			name: "no probes",
			want: Report{Status: StatusServing, Version: "v1", StartedAt: started, Now: later},
		},
		{
			name:   "all probes pass",
			probes: []Probe{fakeProbe{name: "db"}, fakeProbe{name: "media"}},
			want:   Report{Status: StatusServing, Version: "v1", StartedAt: started, Now: later},
		},
		{
			name:   "a failing probe degrades",
			probes: []Probe{fakeProbe{name: "db", err: errDB}, fakeProbe{name: "media"}},
			want: Report{
				Status:    StatusDegraded,
				Version:   "v1",
				StartedAt: started,
				Now:       later,
				Failures:  []ProbeFailure{{Probe: "db", Err: errDB}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := &fakeClock{now: started}
			svc := NewService(clock, "v1", tt.probes...)
			clock.now = later

			got := svc.Check(t.Context())
			if d := cmp.Diff(tt.want, got, cmpopts.EquateErrors()); d != "" {
				t.Errorf("report mismatch (-want +got):\n%s", d)
			}
		})
	}
}
