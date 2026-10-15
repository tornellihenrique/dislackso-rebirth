// Package ice issues the ICE servers voice clients get on join.
package ice

import (
	"context"
	"slices"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

// None issues nothing: clients use only the media server's own STUN-derived
// candidates.
type None struct{}

func (None) Name() string { return "none" }

func (None) Issue(context.Context, domain.ID, time.Duration) ([]ports.IceServer, error) {
	return nil, nil
}

// Static issues one TURN server with fixed credentials, as free TURN
// services that hand out a single username and password expect.
type Static struct {
	URLs       []string
	Username   string
	Credential string
}

func (Static) Name() string { return "static" }

func (s Static) Issue(context.Context, domain.ID, time.Duration) ([]ports.IceServer, error) {
	if len(s.URLs) == 0 {
		return nil, nil
	}
	return []ports.IceServer{{URLs: slices.Clone(s.URLs), Username: s.Username, Credential: s.Credential}}, nil
}
