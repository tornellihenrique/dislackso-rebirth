package ports

import (
	"context"
	"net/http"
	"net/netip"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

// ExposureProvider publishes the hub beyond this machine (ARCHITECTURE §4.2).
type ExposureProvider interface {
	Kind() domain.ExposureKind
	// Run serves handler until ctx ends, calling report whenever its state
	// changes. It returns nil when ctx ends, or the error that stopped it.
	Run(ctx context.Context, handler http.Handler, report func(domain.ExposureStatus)) error
}

// ExposureError is a failure the admin panel can explain.
type ExposureError struct {
	Problem string
	Err     error
}

func (e *ExposureError) Error() string { return e.Problem + ": " + e.Err.Error() }
func (e *ExposureError) Unwrap() error { return e.Err }

// MappedPorts is what the home router agreed to forward.
type MappedPorts struct {
	// "pcp", "nat-pmp" or "upnp".
	Protocol string
	// External is the router's public address with the UDP port.
	External netip.AddrPort
	// Lifetime is how long the mapping lasts; zero is permanent.
	Lifetime time.Duration
	TCP      bool
}

// PortMapper asks the home router to forward the media ports, keeping their
// numbers on the outside.
type PortMapper interface {
	Map(ctx context.Context, udp, tcp int) (MappedPorts, error)
	Unmap(ctx context.Context)
}

// PublicProbe looks at an address from the internet's side.
type PublicProbe interface {
	// Reachable fails until rawURL answers an HTTPS GET with a 2xx status.
	Reachable(ctx context.Context, rawURL string) error
}
