package direct

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/crypto/acme"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

// fakeCA records the order of the steps a DNS check takes and signs what it
// is asked to.
type fakeCA struct {
	directory string
	log       *[]string
	mu        *sync.Mutex
	failCheck string
	now       time.Time
}

func (f *fakeCA) note(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	*f.log = append(*f.log, f.directory+" "+s)
}

func (f *fakeCA) Register(context.Context, *acme.Account, func(string) bool) (*acme.Account, error) {
	return nil, acme.ErrAccountAlreadyExists
}

func (f *fakeCA) AuthorizeOrder(context.Context, []acme.AuthzID, ...acme.OrderOption) (*acme.Order, error) {
	return &acme.Order{URI: "order", AuthzURLs: []string{"authz"}}, nil
}

func (f *fakeCA) GetAuthorization(context.Context, string) (*acme.Authorization, error) {
	return &acme.Authorization{URI: "authz", Status: acme.StatusPending, Challenges: []*acme.Challenge{
		{Type: "tls-alpn-01", Token: "alpn"}, {Type: "dns-01", Token: "dns"},
	}}, nil
}

func (f *fakeCA) DNS01ChallengeRecord(token string) (string, error) {
	return f.directory + "-" + token, nil
}

func (f *fakeCA) Accept(_ context.Context, chal *acme.Challenge) (*acme.Challenge, error) {
	f.note("accept " + chal.Type)
	return chal, nil
}

func (f *fakeCA) WaitAuthorization(context.Context, string) (*acme.Authorization, error) {
	if f.failCheck != "" {
		return nil, &acme.AuthorizationError{Identifier: "chat.example.com", Errors: []error{
			&acme.Error{StatusCode: 400, Detail: f.failCheck},
		}}
	}
	return &acme.Authorization{Status: acme.StatusValid}, nil
}

func (f *fakeCA) RevokeAuthorization(context.Context, string) error {
	f.note("give back")
	return nil
}

func (f *fakeCA) WaitOrder(context.Context, string) (*acme.Order, error) {
	return &acme.Order{URI: "order", FinalizeURL: "finalize"}, nil
}

func (f *fakeCA) CreateOrderCert(_ context.Context, _ string, csr []byte, _ bool) ([][]byte, string, error) {
	f.note("issue")
	req, err := x509.ParseCertificateRequest(csr)
	if err != nil {
		return nil, "", err
	}
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: req.DNSNames[0]}, DNSNames: req.DNSNames,
		NotBefore: f.now.Add(-time.Hour), NotAfter: f.now.Add(90 * 24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, req.PublicKey, caKey)
	return [][]byte{der}, "", err
}

// fakeTXT is DuckDNS's TXT record as the check sees it.
type fakeTXT struct {
	mu      sync.Mutex
	log     *[]string
	value   string
	visible bool
}

func (f *fakeTXT) SetTXT(_ context.Context, name, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	*f.log = append(*f.log, "txt "+name+" "+value)
	f.value = value
	return nil
}

func (f *fakeTXT) ClearTXT(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	*f.log = append(*f.log, "clear "+name)
	f.value = ""
	return nil
}

func (f *fakeTXT) lookup(_ context.Context, name string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.visible || name != "_acme-challenge.chat.example.com" || f.value == "" {
		return nil, nil
	}
	return []string{f.value}, nil
}

func newDNSCerts(t *testing.T, txt *fakeTXT, log *[]string, failCheck string, now time.Time) *dnsCerts {
	t.Helper()
	var mu sync.Mutex
	txt.log = log
	return &dnsCerts{
		domain: "chat.example.com", txtDomain: "team.duckdns.org", dir: t.TempDir(),
		txt: txt, lookupTXT: txt.lookup, logger: slog.New(slog.DiscardHandler),
		directories: []string{"staging", "production"},
		newCA: func(directory string, _ crypto.Signer) acmeCA {
			return &fakeCA{directory: directory, log: log, mu: &mu, failCheck: failCheck, now: now}
		},
		txtWait: 20 * time.Millisecond, txtPoll: time.Millisecond,
	}
}

func TestDNSCertsRunStagingFirstAndKeepTheCertificate(t *testing.T) {
	now := time.Now()
	var log []string
	c := newDNSCerts(t, &fakeTXT{visible: true}, &log, "", now)
	if err := c.ensure(t.Context(), now); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"txt team.duckdns.org staging-dns", "staging accept dns-01", "clear team.duckdns.org", "staging issue",
		"txt team.duckdns.org production-dns", "production accept dns-01", "clear team.duckdns.org", "production issue",
	}
	if diff := cmp.Diff(want, log); diff != "" {
		t.Errorf("steps (-want +got):\n%s", diff)
	}
	if c.cert == nil || c.cert.Leaf.VerifyHostname("chat.example.com") != nil {
		t.Fatalf("certificate = %v", c.cert)
	}

	// A new run starts from the saved certificate and asks nobody.
	log = nil
	again := newDNSCerts(t, &fakeTXT{visible: true}, &log, "", now)
	again.dir = c.dir
	if err := again.ensure(t.Context(), now); err != nil {
		t.Fatal(err)
	}
	if len(log) != 0 || again.cert == nil {
		t.Errorf("a saved certificate was requested again: %v", log)
	}
	// Once due, it is requested again.
	if err := again.ensure(t.Context(), now.Add(70*24*time.Hour)); err != nil || len(log) == 0 {
		t.Errorf("a due certificate was not renewed: err %v, steps %v", err, log)
	}
}

func TestDNSCertsStopBeforeTheCheckWhenTheRecordDoesNotShow(t *testing.T) {
	now := time.Now()
	var log []string
	c := newDNSCerts(t, &fakeTXT{}, &log, "", now)
	err := c.ensure(t.Context(), now)
	if _, ok := errors.AsType[*txtMissingError](err); !ok {
		t.Fatalf("err = %v, want a missing record", err)
	}
	want := []string{"txt team.duckdns.org staging-dns", "staging give back", "clear team.duckdns.org"}
	if diff := cmp.Diff(want, log); diff != "" {
		t.Errorf("steps (-want +got):\n%s", diff)
	}
	if got := certificateError(err, "", now); got.Problem != domain.ProblemDNSRecord ||
		!strings.Contains(got.Err.Error(), "_acme-challenge.chat.example.com") {
		t.Errorf("certificateError = %s: %v", got.Problem, got.Err)
	}
}

func TestDNSCertsReportTheFailedCheck(t *testing.T) {
	now := time.Now()
	var log []string
	c := newDNSCerts(t, &fakeTXT{visible: true}, &log, "No TXT record found at _acme-challenge.chat.example.com", now)
	err := c.ensure(t.Context(), now)
	got := certificateError(err, "", now)
	if got.Problem != domain.ProblemCertificateCheck || got.RetryAfter != failedCheckWait ||
		got.Err.Error() != "No TXT record found at _acme-challenge.chat.example.com" {
		t.Errorf("certificateError = %s, %v: %v", got.Problem, got.RetryAfter, got.Err)
	}
	for _, step := range log {
		if strings.HasPrefix(step, "production") {
			t.Errorf("production was asked after staging failed: %v", log)
		}
	}
}

func TestDue(t *testing.T) {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	leaf := &x509.Certificate{NotBefore: start, NotAfter: start.Add(90 * 24 * time.Hour)}
	if due(leaf, start.Add(59*24*time.Hour)) {
		t.Error("due with more than a third left")
	}
	if !due(leaf, start.Add(60*24*time.Hour)) {
		t.Error("not due with a third left")
	}
}
