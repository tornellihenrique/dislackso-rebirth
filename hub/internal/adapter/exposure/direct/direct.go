// Package direct publishes the hub on this machine's own addresses: on a
// public address with a Let's Encrypt certificate (Direct), or on the local
// network with a self-signed one (LAN).
package direct

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/ingress"
)

const (
	drainTimeout = 5 * time.Second
	// A home connection's public address changes rarely; a few minutes of
	// a stale name after a change is the cost of not asking constantly.
	defaultDNSCheck = 5 * time.Minute
	// Let's Encrypt allows a name 5 failed checks an hour; retrying sooner
	// only ends in its rate limit.
	failedCheckWait = 15 * time.Minute
)

// Public serves on a public address with a certificate from Let's Encrypt:
// over its DNS check when the dynamic DNS provider can publish TXT records,
// else over the TLS-ALPN check on the same port.
type Public struct {
	Domain   string
	Email    string
	Addr     string
	CacheDir string
	// DNS keeps DNSName pointed at this machine: Domain itself, or the name
	// Domain is a CNAME to. It is nil when the domain's own DNS already
	// points here. PublicIP is this machine's address as the internet sees it.
	DNS      ports.DynamicDNS
	DNSName  string
	PublicIP func(ctx context.Context) (netip.Addr, error)
	DNSCheck time.Duration
	// Challenge publishes the DNS check's TXT record under DNSName, so the
	// certificate needs no inbound connection; LookupTXT reads public DNS.
	Challenge ports.DNSChallenge
	LookupTXT func(ctx context.Context, name string) ([]string, error)
	Logger    *slog.Logger
	Clock     ports.Clock

	// certs replaces Let's Encrypt in tests.
	certs certSource
}

// certSource is what Run needs from autocert.Manager.
type certSource interface {
	GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error)
	TLSConfig() *tls.Config
}

func (p *Public) Kind() domain.ExposureKind { return domain.ExposureDirect }

func (p *Public) Run(ctx context.Context, handler http.Handler, report func(domain.ExposureStatus)) error {
	// A failed run is retried with a new Run; what this one started must not
	// stay behind.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	starting := func(stage string) {
		report(domain.ExposureStatus{
			Kind: domain.ExposureDirect, State: domain.ExposureStarting, Stage: stage,
			URL: "https://" + hostPort(p.Domain, p.Addr), Since: p.Clock.Now(),
		})
	}
	if p.DNS != nil {
		// Let's Encrypt looks the domain up before it issues the certificate.
		starting(domain.StageDNS)
		ip, err := p.point(ctx)
		if err != nil {
			return &ports.ExposureError{Problem: domain.ProblemDynamicDNS, Err: err}
		}
		go p.keepPointed(ctx, ip)
	}
	starting(domain.StageCertificate)
	if p.Challenge != nil {
		return p.serveDNSChecked(ctx, handler, report)
	}
	certs := p.certs
	var checks *checkLog
	if certs == nil {
		checks = &checkLog{base: http.DefaultTransport}
		certs = &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(p.Domain),
			Cache:      autocert.DirCache(p.CacheDir),
			Email:      p.Email,
			Client: &acme.Client{
				DirectoryURL: autocert.DefaultACMEDirectory, RetryBackoff: acmeBackoff,
				HTTPClient: &http.Client{Transport: checks},
			},
		}
	}
	ln, err := listen(ctx, p.Addr)
	if err != nil {
		return err
	}
	// Let's Encrypt checks the domain with a TLS handshake on this port
	// (TLS-ALPN), so it must be served while the certificate is requested.
	served := make(chan error, 1)
	go func() {
		info := ingress.Info{Name: string(domain.ExposureDirect), Secure: true, HSTS: true}
		served <- serve(ctx, tls.NewListener(ln, certs.TLSConfig()), handler, info, p.Logger)
	}()
	// The first certificate is requested now, so a wrong domain or a closed
	// port shows in the admin panel instead of failing the first visitor.
	if _, err := certs.GetCertificate(browserHello(p.Domain)); err != nil {
		cancel()
		<-served
		return certificateError(err, checks.last(), p.Clock.Now())
	}
	report(domain.ExposureStatus{
		Kind: domain.ExposureDirect, State: domain.ExposureRunning, Since: p.Clock.Now(),
		URL: "https://" + hostPort(p.Domain, p.Addr),
	})
	return <-served
}

func (p *Public) serveDNSChecked(ctx context.Context, handler http.Handler, report func(domain.ExposureStatus)) error {
	certs := &dnsCerts{
		domain: p.Domain, txtDomain: p.DNSName, email: p.Email, dir: p.CacheDir,
		txt: p.Challenge, lookupTXT: p.LookupTXT, logger: p.Logger,
	}
	if err := certs.ensure(ctx, p.Clock.Now()); err != nil {
		return certificateError(err, "", p.Clock.Now())
	}
	ln, err := listen(ctx, p.Addr)
	if err != nil {
		return err
	}
	go certs.renew(ctx, p.Clock)
	report(domain.ExposureStatus{
		Kind: domain.ExposureDirect, State: domain.ExposureRunning, Since: p.Clock.Now(),
		URL: "https://" + hostPort(p.Domain, p.Addr),
	})
	info := ingress.Info{Name: string(domain.ExposureDirect), Secure: true, HSTS: true}
	return serve(ctx, tls.NewListener(ln, certs.tlsConfig()), handler, info, p.Logger)
}

// acmeBackoff retries Let's Encrypt's passing errors briefly and gives up
// on a rate limit at once. The ACME client would otherwise sleep through its
// Retry-After, which can be an hour, inside one certificate request.
func acmeBackoff(n int, _ *http.Request, res *http.Response) time.Duration {
	if n > 3 || res != nil && res.StatusCode == http.StatusTooManyRequests {
		return 0
	}
	return time.Duration(n) * time.Second
}

// certificateError explains a failed certificate request; failedCheck is
// Let's Encrypt's reason when its check of the domain failed and the error
// does not carry it.
func certificateError(err error, failedCheck string, now time.Time) *ports.ExposureError {
	if ae, ok := errors.AsType[*acme.Error](err); ok && ae.StatusCode == http.StatusTooManyRequests {
		return &ports.ExposureError{
			Problem: domain.ProblemCertificateLimit, Err: err,
			RetryAfter: retryAfter(ae.Header.Get("Retry-After"), now),
		}
	}
	if _, ok := errors.AsType[*publishError](err); ok {
		return &ports.ExposureError{Problem: domain.ProblemDynamicDNS, Err: err}
	}
	if _, ok := errors.AsType[*txtMissingError](err); ok {
		return &ports.ExposureError{Problem: domain.ProblemDNSRecord, Err: err}
	}
	if failedCheck == "" {
		failedCheck = authorizationReason(err)
	}
	if failedCheck != "" {
		return &ports.ExposureError{
			Problem: domain.ProblemCertificateCheck, Err: &checkError{reason: failedCheck, err: err},
			RetryAfter: failedCheckWait,
		}
	}
	return &ports.ExposureError{Problem: domain.ProblemCertificate, Err: err}
}

// authorizationReason is the CA's reason in a failed authorization, or "".
func authorizationReason(err error) string {
	ae, ok := errors.AsType[*acme.AuthorizationError](err)
	if !ok {
		return ""
	}
	for _, e := range ae.Errors {
		if pe, ok := errors.AsType[*acme.Error](e); ok && pe.Detail != "" {
			return pe.Detail
		}
		if e != nil {
			return e.Error()
		}
	}
	return ""
}

// retryAfter reads a Retry-After header, in seconds or as a date.
func retryAfter(v string, now time.Time) time.Duration {
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(max(secs, 0)) * time.Second
	}
	if at, err := http.ParseTime(v); err == nil {
		return max(at.Sub(now), 0)
	}
	return 0
}

// browserHello asks for the ECDSA certificate browsers get; autocert would
// issue an RSA one for a bare hello, and a second one for the first visitor.
func browserHello(serverName string) *tls.ClientHelloInfo {
	return &tls.ClientHelloInfo{
		ServerName:       serverName,
		SignatureSchemes: []tls.SignatureScheme{tls.ECDSAWithP256AndSHA256},
		SupportedCurves:  []tls.CurveID{tls.CurveP256},
		CipherSuites:     []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256},
	}
}

func (p *Public) point(ctx context.Context) (netip.Addr, error) {
	ip, err := p.PublicIP(ctx)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("find the public address: %w", err)
	}
	if err := p.DNS.Point(ctx, p.DNSName, ip); err != nil {
		return netip.Addr{}, err
	}
	return ip, nil
}

// keepPointed follows the public address while the hub serves, so a new
// one after a router restart reaches the domain.
func (p *Public) keepPointed(ctx context.Context, last netip.Addr) {
	every := p.DNSCheck
	if every <= 0 {
		every = defaultDNSCheck
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		ip, err := p.PublicIP(ctx)
		if err != nil || ip == last {
			continue
		}
		if err := p.DNS.Point(ctx, p.DNSName, ip); err != nil {
			p.Logger.WarnContext(ctx, "point the domain at the new public address",
				slog.String("domain", p.DNSName), slog.Any("error", err))
			continue
		}
		p.Logger.InfoContext(ctx, "the domain points at the new public address",
			slog.String("domain", p.DNSName), slog.String("ip", ip.String()))
		last = ip
	}
}

// LAN serves on this machine's network with a self-signed certificate,
// created once and kept in CertDir. Browsers warn about it once per device.
type LAN struct {
	Addr    string
	CertDir string
	Logger  *slog.Logger
	Clock   ports.Clock
}

func (l *LAN) Kind() domain.ExposureKind { return domain.ExposureLAN }

func (l *LAN) Run(ctx context.Context, handler http.Handler, report func(domain.ExposureStatus)) error {
	report(domain.ExposureStatus{Kind: domain.ExposureLAN, State: domain.ExposureStarting, Since: l.Clock.Now()})
	ips := lanIPs()
	cert, err := selfSigned(l.CertDir, ips)
	if err != nil {
		return &ports.ExposureError{Problem: domain.ProblemCertificate, Err: err}
	}
	ln, err := listen(ctx, l.Addr)
	if err != nil {
		return err
	}
	host := "localhost"
	if len(ips) > 0 {
		host = ips[0].String()
	}
	report(domain.ExposureStatus{
		Kind: domain.ExposureLAN, State: domain.ExposureRunning, Since: l.Clock.Now(),
		URL: "https://" + hostPort(host, l.Addr),
	})
	tlsLn := tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	return serve(ctx, tlsLn, handler, ingress.Info{Name: string(domain.ExposureLAN), Secure: true}, l.Logger)
}

func listen(ctx context.Context, addr string) (net.Listener, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return nil, &ports.ExposureError{Problem: domain.ProblemAddressInUse, Err: err}
	}
	return ln, nil
}

func serve(ctx context.Context, ln net.Listener, handler http.Handler, info ingress.Info, logger *slog.Logger) error {
	if err := ingress.Serve(ctx, ln, handler, info, logger, drainTimeout); err != nil && ctx.Err() == nil {
		return &ports.ExposureError{Problem: domain.ProblemExposureFailed, Err: err}
	}
	return nil
}

// hostPort leaves the default HTTPS port out of URLs.
func hostPort(host, addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "443" {
		return host
	}
	return net.JoinHostPort(host, port)
}

// lanIPs lists this machine's private IPv4 addresses, the ones LAN peers use.
func lanIPs() []net.IP {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []net.IP
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok {
			if ip4 := n.IP.To4(); ip4 != nil && ip4.IsPrivate() {
				out = append(out, ip4)
			}
		}
	}
	return out
}

// selfSigned loads the LAN certificate, or creates one valid for ips. A
// certificate that lacks a current address is replaced.
func selfSigned(dir string, ips []net.IP) (tls.Certificate, error) {
	certPath, keyPath := filepath.Join(dir, "lan-cert.pem"), filepath.Join(dir, "lan-key.pem")
	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil && covers(cert, ips) {
		return cert, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return tls.Certificate{}, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "Dislackso hub"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.AddDate(2, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  append([]net.IP{net.IPv4(127, 0, 0, 1)}, ips...),
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := errors.Join(os.WriteFile(certPath, certPEM, 0o600), os.WriteFile(keyPath, keyPEM, 0o600)); err != nil {
		return tls.Certificate{}, fmt.Errorf("save LAN certificate: %w", err)
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

func covers(cert tls.Certificate, ips []net.IP) bool {
	if len(cert.Certificate) == 0 {
		return false
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil || time.Now().After(leaf.NotAfter.Add(-30*24*time.Hour)) {
		return false
	}
	for _, ip := range ips {
		if leaf.VerifyHostname(ip.String()) != nil {
			return false
		}
	}
	return true
}
