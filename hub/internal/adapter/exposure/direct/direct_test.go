package direct

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
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

// fakeACME stands in for Let's Encrypt: the certificate is issued only if a
// TLS-ALPN handshake on the hub's port completes while it is requested.
type fakeACME struct {
	addr string
	cert tls.Certificate
}

const acmeALPN = "acme-tls/1"

func (f *fakeACME) TLSConfig() *tls.Config {
	return &tls.Config{
		NextProtos:     []string{acmeALPN},
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return &f.cert, nil },
		MinVersion:     tls.VersionTLS12,
	}
}

func (f *fakeACME) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	d := tls.Dialer{Config: &tls.Config{
		ServerName: hello.ServerName, NextProtos: []string{acmeALPN},
		InsecureSkipVerify: true, MinVersion: tls.VersionTLS12,
	}}
	conn, err := d.DialContext(ctx, "tcp", f.addr)
	if err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}
	_ = conn.Close()
	return &f.cert, nil
}

func TestPublicServesWhileTheCertificateIsIssued(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	cert, err := selfSigned(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	p := &Public{
		Domain: "hub.example.test", Addr: addr, certs: &fakeACME{addr: addr, cert: cert},
		Logger: slog.New(slog.DiscardHandler), Clock: fakes.NewClock(),
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	running := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- p.Run(ctx, http.NotFoundHandler(), func(s domain.ExposureStatus) {
			if s.State == domain.ExposureRunning {
				running <- struct{}{}
			}
		})
	}()
	select {
	case <-running:
	case err := <-done:
		t.Fatalf("Run = %v, want it running", err)
	}
	cancel()
	if err := <-done; err != nil {
		t.Errorf("Run after cancel = %v", err)
	}
}
