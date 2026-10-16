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
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/ingress"
)

const drainTimeout = 5 * time.Second

// Public serves on a public address with a certificate from Let's Encrypt,
// over the TLS-ALPN challenge on the same port.
type Public struct {
	Domain   string
	Email    string
	Addr     string
	CacheDir string
	Logger   *slog.Logger
	Clock    ports.Clock
}

func (p *Public) Kind() domain.ExposureKind { return domain.ExposureDirect }

func (p *Public) Run(ctx context.Context, handler http.Handler, report func(domain.ExposureStatus)) error {
	report(domain.ExposureStatus{Kind: domain.ExposureDirect, State: domain.ExposureStarting, Since: p.Clock.Now()})
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(p.Domain),
		Cache:      autocert.DirCache(p.CacheDir),
		Email:      p.Email,
	}
	ln, err := listen(ctx, p.Addr)
	if err != nil {
		return err
	}
	// The first certificate is requested now, so a wrong domain or a closed
	// port shows in the admin panel instead of failing the first visitor.
	if _, err := m.GetCertificate(&tls.ClientHelloInfo{ServerName: p.Domain}); err != nil {
		_ = ln.Close()
		return &ports.ExposureError{Problem: domain.ProblemCertificate, Err: err}
	}
	report(domain.ExposureStatus{
		Kind: domain.ExposureDirect, State: domain.ExposureRunning, Since: p.Clock.Now(),
		URL: "https://" + hostPort(p.Domain, p.Addr),
	})
	info := ingress.Info{Name: string(domain.ExposureDirect), Secure: true, HSTS: true}
	return serve(ctx, tls.NewListener(ln, m.TLSConfig()), handler, info, p.Logger)
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
