package direct

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/crypto/acme"

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

func TestCertificateError(t *testing.T) {
	now := time.Date(2026, 10, 18, 3, 0, 0, 0, time.UTC)
	limited := func(retryAfter string) error {
		return fmt.Errorf("authorize: %w", &acme.Error{
			StatusCode: http.StatusTooManyRequests, Detail: "too many failed authorizations",
			Header: http.Header{"Retry-After": {retryAfter}},
		})
	}
	noViable := errors.New("acme/autocert: no viable challenge type found")
	const refused = "186.195.214.169: Connection refused"
	tests := []struct {
		name        string
		err         error
		failedCheck string
		problem     string
		wait        time.Duration
		detail      string
	}{
		{"rate limit in seconds", limited("720"), "", domain.ProblemCertificateLimit, 12 * time.Minute, ""},
		{"rate limit as a date", limited("Sun, 18 Oct 2026 03:35:00 GMT"), "", domain.ProblemCertificateLimit, 35 * time.Minute, ""},
		{"rate limit without a time", limited(""), "", domain.ProblemCertificateLimit, 0, ""},
		{"failed check", noViable, refused, domain.ProblemCertificateCheck, failedCheckWait, refused},
		{"refused order", &acme.Error{StatusCode: http.StatusForbidden}, "", domain.ProblemCertificate, 0, ""},
		{"no answer", errors.New("context deadline exceeded"), "", domain.ProblemCertificate, 0, ""},
	}
	for _, tt := range tests {
		got := certificateError(tt.err, tt.failedCheck, now)
		if got.Problem != tt.problem || got.RetryAfter != tt.wait || !errors.Is(got, tt.err) {
			t.Errorf("%s: certificateError = %s, %v; want %s, %v", tt.name, got.Problem, got.RetryAfter, tt.problem, tt.wait)
		}
		if tt.detail != "" && got.Err.Error() != tt.detail {
			t.Errorf("%s: detail = %q, want %q", tt.name, got.Err.Error(), tt.detail)
		}
	}
}

func TestCheckLogKeepsLetsEncryptsReason(t *testing.T) {
	const invalid = `{"identifier":{"type":"dns","value":"team.duckdns.org"},"status":"invalid",
		"challenges":[{"type":"tls-alpn-01","status":"invalid",
		"error":{"type":"urn:ietf:params:acme:error:connection","detail":"186.195.214.169: Timeout during connect (likely firewall problem)"}}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/authz/") {
			_, _ = io.WriteString(w, invalid)
			return
		}
		_, _ = io.WriteString(w, `{"status":"pending"}`)
	}))
	defer srv.Close()
	log := &checkLog{base: http.DefaultTransport}
	client := &http.Client{Transport: log}
	for _, path := range []string{"/acme/order/1", "/acme/authz/1/2"} {
		res, err := client.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if path == "/acme/authz/1/2" && string(body) != invalid {
			t.Errorf("the authorization reached autocert changed: %s", body)
		}
	}
	if got, want := log.last(), "186.195.214.169: Timeout during connect (likely firewall problem)"; got != want {
		t.Errorf("last = %q, want %q", got, want)
	}
	if got := (*checkLog)(nil).last(); got != "" {
		t.Errorf("nil log: last = %q", got)
	}
	if got := failedCheck([]byte(`{"status":"valid","challenges":[{"error":{"detail":"old"}}]}`)); got != "" {
		t.Errorf("a valid authorization has no failed check, got %q", got)
	}
}

func TestACMEBackoffGivesUpOnRateLimits(t *testing.T) {
	limited := &http.Response{StatusCode: http.StatusTooManyRequests}
	unavailable := &http.Response{StatusCode: http.StatusServiceUnavailable}
	if d := acmeBackoff(1, nil, limited); d != 0 {
		t.Errorf("rate limited: backoff = %v, want none", d)
	}
	if d := acmeBackoff(1, nil, unavailable); d <= 0 {
		t.Errorf("unavailable: backoff = %v, want a retry", d)
	}
	if d := acmeBackoff(4, nil, unavailable); d != 0 {
		t.Errorf("fourth retry: backoff = %v, want none", d)
	}
}
