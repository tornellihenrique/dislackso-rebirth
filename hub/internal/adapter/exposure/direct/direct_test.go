package direct

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

type fakeDNS struct {
	mu     sync.Mutex
	err    error
	points []netip.Addr
	called chan struct{}
}

func (f *fakeDNS) Point(_ context.Context, _ string, ip netip.Addr) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.points = append(f.points, ip)
	if f.called != nil {
		f.called <- struct{}{}
	}
	return nil
}

func (f *fakeDNS) pointed() []netip.Addr {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]netip.Addr(nil), f.points...)
}

func TestPublicDNSFailureStopsBeforeTheCertificate(t *testing.T) {
	tests := []struct {
		name     string
		dns      *fakeDNS
		publicIP func(context.Context) (netip.Addr, error)
	}{
		{
			name:     "no public address",
			dns:      &fakeDNS{},
			publicIP: func(context.Context) (netip.Addr, error) { return netip.Addr{}, errors.New("no STUN server answered") },
		},
		{
			name:     "update rejected",
			dns:      &fakeDNS{err: errors.New("rejected")},
			publicIP: func(context.Context) (netip.Addr, error) { return netip.MustParseAddr("203.0.113.7"), nil },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Public{
				Domain: "team.duckdns.org", Addr: "127.0.0.1:0", CacheDir: t.TempDir(),
				DNS: tt.dns, PublicIP: tt.publicIP,
				Logger: slog.New(slog.DiscardHandler), Clock: fakes.NewClock(),
			}
			var stages []string
			err := p.Run(t.Context(), http.NotFoundHandler(), func(s domain.ExposureStatus) { stages = append(stages, s.Stage) })
			var ee *ports.ExposureError
			if !errors.As(err, &ee) || ee.Problem != domain.ProblemDynamicDNS {
				t.Fatalf("err = %v, want problem %s", err, domain.ProblemDynamicDNS)
			}
			if diff := cmp.Diff([]string{domain.StageDNS}, stages); diff != "" {
				t.Errorf("stages (-want +got):\n%s", diff)
			}
		})
	}
}

func TestKeepPointedFollowsTheAddress(t *testing.T) {
	first, second := netip.MustParseAddr("203.0.113.7"), netip.MustParseAddr("203.0.113.8")
	answers := make(chan netip.Addr, 4)
	answers <- first // unchanged: no update
	answers <- second
	answers <- second // unchanged again
	dns := &fakeDNS{called: make(chan struct{}, 4)}
	p := &Public{
		Domain: "team.duckdns.org", DNS: dns, DNSCheck: time.Millisecond,
		PublicIP: func(ctx context.Context) (netip.Addr, error) {
			select {
			case ip := <-answers:
				return ip, nil
			case <-ctx.Done():
				return netip.Addr{}, ctx.Err()
			}
		},
		Logger: slog.New(slog.DiscardHandler), Clock: fakes.NewClock(),
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		p.keepPointed(ctx, first)
		close(done)
	}()
	<-dns.called
	for len(answers) > 0 {
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	if diff := cmp.Diff([]netip.Addr{second}, dns.pointed(), cmp.Comparer(func(a, b netip.Addr) bool { return a == b })); diff != "" {
		t.Errorf("points (-want +got):\n%s", diff)
	}
}
