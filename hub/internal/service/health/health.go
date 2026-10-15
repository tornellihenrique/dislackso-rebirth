// Package health reports whether the hub can serve requests.
package health

import (
	"context"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

const probeTimeout = 2 * time.Second

// Probe checks one dependency the hub needs to serve requests, such as the
// database, the media server or the exposure provider.
type Probe interface {
	Name() string
	Check(ctx context.Context) error
}

type Status int

const (
	StatusServing Status = iota + 1
	// StatusDegraded means the process is up but at least one probe fails.
	StatusDegraded
)

type ProbeFailure struct {
	Probe string
	Err   error
}

type Report struct {
	Status    Status
	Version   string
	StartedAt time.Time
	Now       time.Time
	Failures  []ProbeFailure
}

type Service struct {
	clock     ports.Clock
	version   string
	startedAt time.Time
	probes    []Probe
}

func NewService(clock ports.Clock, version string, probes ...Probe) *Service {
	return &Service{
		clock:     clock,
		version:   version,
		startedAt: clock.Now(),
		probes:    probes,
	}
}

// Check runs every probe and summarizes the result.
func (s *Service) Check(ctx context.Context) Report {
	r := Report{
		Status:    StatusServing,
		Version:   s.version,
		StartedAt: s.startedAt,
	}
	for _, p := range s.probes {
		if err := runProbe(ctx, p); err != nil {
			r.Failures = append(r.Failures, ProbeFailure{Probe: p.Name(), Err: err})
		}
	}
	if len(r.Failures) > 0 {
		r.Status = StatusDegraded
	}
	r.Now = s.clock.Now()
	return r
}

func runProbe(ctx context.Context, p Probe) error {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	return p.Check(ctx)
}
