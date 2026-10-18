package direct

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"golang.org/x/crypto/acme"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

const (
	letsEncryptStaging = "https://acme-staging-v02.api.letsencrypt.org/directory"
	// One certificate request, both runs included, gives up after this.
	obtainTimeout = 10 * time.Minute
	// DuckDNS answers new TXT values within seconds; public resolvers may
	// hold the previous one for its TTL.
	defaultTXTWait  = 2 * time.Minute
	defaultTXTPoll  = 5 * time.Second
	renewCheckEvery = 12 * time.Hour
)

// acmeCA is the part of acme.Client a DNS check uses.
type acmeCA interface {
	Register(ctx context.Context, acct *acme.Account, prompt func(tosURL string) bool) (*acme.Account, error)
	AuthorizeOrder(ctx context.Context, id []acme.AuthzID, opt ...acme.OrderOption) (*acme.Order, error)
	GetAuthorization(ctx context.Context, url string) (*acme.Authorization, error)
	DNS01ChallengeRecord(token string) (string, error)
	Accept(ctx context.Context, chal *acme.Challenge) (*acme.Challenge, error)
	WaitAuthorization(ctx context.Context, url string) (*acme.Authorization, error)
	RevokeAuthorization(ctx context.Context, url string) error
	WaitOrder(ctx context.Context, url string) (*acme.Order, error)
	CreateOrderCert(ctx context.Context, url string, csr []byte, bundle bool) ([][]byte, string, error)
}

// dnsCerts gets and renews the certificate over Let's Encrypt's DNS check:
// the TXT record goes out through the dynamic DNS provider, so no inbound
// connection is needed, which some home providers filter from abroad.
// Every request first runs on Let's Encrypt's staging server, so a setup
// that cannot pass never spends the production allowance.
type dnsCerts struct {
	domain string
	// txtDomain is the dynamic DNS name whose TXT record answers the check;
	// an own domain reaches it with a CNAME from _acme-challenge.<domain>.
	txtDomain string
	email     string
	dir       string
	txt       ports.DNSChallenge
	lookupTXT func(ctx context.Context, name string) ([]string, error)
	logger    *slog.Logger

	// Overridden in tests.
	directories []string
	newCA       func(directory string, key crypto.Signer) acmeCA
	txtWait     time.Duration
	txtPoll     time.Duration

	mu   sync.RWMutex
	cert *tls.Certificate
}

func (c *dnsCerts) tlsConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"h2", "http/1.1"},
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			c.mu.RLock()
			defer c.mu.RUnlock()
			if c.cert == nil {
				return nil, errors.New("no certificate yet")
			}
			return c.cert, nil
		},
	}
}

// ensure loads the saved certificate, or gets a new one when there is none
// or it is due for renewal.
func (c *dnsCerts) ensure(ctx context.Context, now time.Time) error {
	if cert, err := c.load(); err == nil && !due(cert.Leaf, now) {
		c.set(cert)
		return nil
	}
	cert, err := c.obtain(ctx)
	if err != nil {
		return err
	}
	if err := c.save(cert); err != nil {
		c.logger.WarnContext(ctx, "save the certificate", slog.Any("error", err))
	}
	c.set(cert)
	return nil
}

// renew keeps the certificate current while the hub serves; a failed
// renewal is retried at the next check, long before the certificate ends.
func (c *dnsCerts) renew(ctx context.Context, clock ports.Clock) {
	t := time.NewTicker(renewCheckEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		c.mu.RLock()
		leaf := c.cert.Leaf
		c.mu.RUnlock()
		if !due(leaf, clock.Now()) {
			continue
		}
		if err := c.ensure(ctx, clock.Now()); err != nil {
			c.logger.WarnContext(ctx, "renew the certificate", slog.String("domain", c.domain), slog.Any("error", err))
		}
	}
}

func (c *dnsCerts) set(cert *tls.Certificate) {
	c.mu.Lock()
	c.cert = cert
	c.mu.Unlock()
}

// due is true once a third of the certificate's lifetime is left, whatever
// lifetime Let's Encrypt issues.
func due(leaf *x509.Certificate, now time.Time) bool {
	life := leaf.NotAfter.Sub(leaf.NotBefore)
	return !now.Before(leaf.NotAfter.Add(-life / 3))
}

func (c *dnsCerts) obtain(ctx context.Context) (*tls.Certificate, error) {
	ctx, cancel := context.WithTimeout(ctx, obtainTimeout)
	defer cancel()
	directories := c.directories
	if directories == nil {
		directories = []string{letsEncryptStaging, acme.LetsEncryptURL}
	}
	var cert *tls.Certificate
	for _, dir := range directories {
		var err error
		if cert, err = c.obtainFrom(ctx, dir); err != nil {
			return nil, err
		}
	}
	return cert, nil
}

func (c *dnsCerts) obtainFrom(ctx context.Context, directory string) (*tls.Certificate, error) {
	key, err := c.accountKey(directory)
	if err != nil {
		return nil, err
	}
	ca := c.ca(directory, key)
	var contact []string
	if c.email != "" {
		contact = []string{"mailto:" + c.email}
	}
	if _, err := ca.Register(ctx, &acme.Account{Contact: contact}, acme.AcceptTOS); err != nil &&
		!errors.Is(err, acme.ErrAccountAlreadyExists) {
		return nil, fmt.Errorf("register with %s: %w", directory, err)
	}
	order, err := ca.AuthorizeOrder(ctx, acme.DomainIDs(c.domain))
	if err != nil {
		return nil, err
	}
	for _, u := range order.AuthzURLs {
		if err := c.authorize(ctx, ca, u); err != nil {
			return nil, err
		}
	}
	if order, err = ca.WaitOrder(ctx, order.URI); err != nil {
		return nil, err
	}
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{DNSNames: []string{c.domain}}, certKey)
	if err != nil {
		return nil, err
	}
	der, _, err := ca.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	if err != nil {
		return nil, err
	}
	if len(der) == 0 {
		return nil, errors.New("the certificate came back empty")
	}
	leaf, err := x509.ParseCertificate(der[0])
	if err != nil {
		return nil, err
	}
	return &tls.Certificate{Certificate: der, PrivateKey: certKey, Leaf: leaf}, nil
}

func (c *dnsCerts) ca(directory string, key crypto.Signer) acmeCA {
	if c.newCA != nil {
		return c.newCA(directory, key)
	}
	return &acme.Client{Key: key, DirectoryURL: directory, RetryBackoff: acmeBackoff, UserAgent: "dislackso"}
}

func (c *dnsCerts) authorize(ctx context.Context, ca acmeCA, url string) error {
	z, err := ca.GetAuthorization(ctx, url)
	if err != nil {
		return err
	}
	if z.Status == acme.StatusValid {
		return nil
	}
	i := slices.IndexFunc(z.Challenges, func(ch *acme.Challenge) bool { return ch.Type == "dns-01" })
	if i < 0 {
		return errors.New("let's encrypt offered no DNS check")
	}
	chal := z.Challenges[i]
	value, err := ca.DNS01ChallengeRecord(chal.Token)
	if err != nil {
		return err
	}
	if err := c.txt.SetTXT(ctx, c.txtDomain, value); err != nil {
		return &publishError{err: err}
	}
	defer func() {
		if err := c.txt.ClearTXT(context.WithoutCancel(ctx), c.txtDomain); err != nil {
			c.logger.WarnContext(ctx, "clear the DNS check record", slog.Any("error", err))
		}
	}()
	name := "_acme-challenge." + c.domain
	if err := c.waitTXT(ctx, name, value); err != nil {
		// Nobody checked it, so it does not count as a failure; give it back.
		_ = ca.RevokeAuthorization(context.WithoutCancel(ctx), url)
		return err
	}
	if _, err := ca.Accept(ctx, chal); err != nil {
		return err
	}
	_, err = ca.WaitAuthorization(ctx, z.URI)
	return err
}

// waitTXT waits until public DNS shows value at name. Let's Encrypt asks the
// authoritative servers itself, but a missing CNAME shows here first, before
// a failed check counts against the name.
func (c *dnsCerts) waitTXT(ctx context.Context, name, value string) error {
	wait, poll := c.txtWait, c.txtPoll
	if wait <= 0 {
		wait = defaultTXTWait
	}
	if poll <= 0 {
		poll = defaultTXTPoll
	}
	for range int(wait / poll) {
		if values, err := c.lookupTXT(ctx, name); err == nil && slices.Contains(values, value) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
	}
	return &txtMissingError{name: name, from: c.txtDomain}
}

// accountKey is this hub's key with the given ACME server, created once.
func (c *dnsCerts) accountKey(directory string) (crypto.Signer, error) {
	file := "dns-account-production.key"
	if directory == letsEncryptStaging {
		file = "dns-account-staging.key"
	}
	path := filepath.Join(c.dir, file)
	if raw, err := os.ReadFile(path); err == nil {
		if b, _ := pem.Decode(raw); b != nil {
			return x509.ParseECPrivateKey(b.Bytes)
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return nil, fmt.Errorf("save the ACME account key: %w", err)
	}
	return key, nil
}

func (c *dnsCerts) certPath() string { return filepath.Join(c.dir, "dns-"+c.domain+".pem") }

func (c *dnsCerts) save(cert *tls.Certificate) error {
	key, ok := cert.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		return errors.New("unexpected certificate key")
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	out := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	for _, d := range cert.Certificate {
		out = append(out, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: d})...)
	}
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(c.certPath(), out, 0o600)
}

func (c *dnsCerts) load() (*tls.Certificate, error) {
	raw, err := os.ReadFile(c.certPath())
	if err != nil {
		return nil, err
	}
	var keyPEM, certPEM []byte
	for b, rest := pem.Decode(raw); b != nil; b, rest = pem.Decode(rest) {
		if b.Type == "CERTIFICATE" {
			certPEM = append(certPEM, pem.EncodeToMemory(b)...)
		} else {
			keyPEM = pem.EncodeToMemory(b)
		}
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	if cert.Leaf == nil || cert.Leaf.VerifyHostname(c.domain) != nil {
		return nil, errors.New("the saved certificate is for another domain")
	}
	return &cert, nil
}

// publishError is the dynamic DNS provider refusing the check's TXT record.
type publishError struct{ err error }

func (e *publishError) Error() string { return "publish the DNS check record: " + e.err.Error() }
func (e *publishError) Unwrap() error { return e.err }

// txtMissingError is the check's TXT record not showing in public DNS.
type txtMissingError struct{ name, from string }

func (e *txtMissingError) Error() string {
	return fmt.Sprintf("%s shows no TXT record from %s", e.name, e.from)
}
