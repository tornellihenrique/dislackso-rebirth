// Package tailscale publishes the hub with Tailscale Funnel through an
// embedded tsnet node: https://<hostname>.<tailnet>.ts.net, no install needed.
package tailscale

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"tailscale.com/ipn"
	"tailscale.com/logtail"
	"tailscale.com/tsnet"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/ingress"
)

const (
	pollInterval = time.Second
	drainTimeout = 5 * time.Second
)

// Provider runs a tsnet node whose state lives in Dir, so it stays signed in
// across restarts.
type Provider struct {
	Dir      string
	Hostname string
	Logger   *slog.Logger
	Clock    ports.Clock
}

func (p *Provider) Kind() domain.ExposureKind { return domain.ExposureTailscaleFunnel }

func (p *Provider) status(state domain.ExposureState) domain.ExposureStatus {
	return domain.ExposureStatus{Kind: domain.ExposureTailscaleFunnel, State: state, Since: p.Clock.Now()}
}

func (p *Provider) Run(ctx context.Context, handler http.Handler, report func(domain.ExposureStatus)) error {
	// The node would otherwise upload its logs to Tailscale.
	logtail.Disable()
	srv := &tsnet.Server{
		Dir:      p.Dir,
		Hostname: p.Hostname,
		Logf:     func(string, ...any) {},
		UserLogf: func(format string, args ...any) {
			p.Logger.DebugContext(ctx, "tailscale: "+fmt.Sprintf(format, args...))
		},
	}
	defer func() { _ = srv.Close() }()
	report(p.status(domain.ExposureStarting))
	if err := srv.Start(); err != nil {
		return &ports.ExposureError{Problem: domain.ProblemExposureFailed, Err: err}
	}
	lc, err := srv.LocalClient()
	if err != nil {
		return &ports.ExposureError{Problem: domain.ProblemExposureFailed, Err: err}
	}

	// Until the node is signed in, show the login link it offers.
	loginURL := ""
	for {
		st, err := lc.StatusWithoutPeers(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return &ports.ExposureError{Problem: domain.ProblemExposureFailed, Err: err}
		}
		if st.BackendState == ipn.Running.String() {
			break
		}
		if st.AuthURL != "" && st.AuthURL != loginURL {
			loginURL = st.AuthURL
			s := p.status(domain.ExposureNeedsLogin)
			s.LoginURL, s.Problem = loginURL, domain.ProblemTailscaleLogin
			report(s)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(pollInterval):
		}
	}

	ln, err := srv.ListenFunnel("tcp", ":443")
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return &ports.ExposureError{Problem: funnelProblem(err), Err: err}
	}
	domains := srv.CertDomains()
	if len(domains) == 0 {
		_ = ln.Close()
		return &ports.ExposureError{Problem: domain.ProblemTailscaleHTTPS, Err: errors.New("no certificate domain")}
	}
	s := p.status(domain.ExposureRunning)
	s.URL = "https://" + domains[0]
	report(s)
	info := ingress.Info{
		Name:        string(domain.ExposureTailscaleFunnel),
		Secure:      true,
		HSTS:        true,
		ClientIP:    clientIP,
		ConnContext: connContext,
	}
	if err := ingress.Serve(ctx, ln, handler, info, p.Logger, drainTimeout); err != nil && ctx.Err() == nil {
		return &ports.ExposureError{Problem: domain.ProblemExposureFailed, Err: err}
	}
	return nil
}

// funnelProblem tells the admin what to change in Tailscale.
func funnelProblem(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "https"):
		return domain.ProblemTailscaleHTTPS
	case strings.Contains(msg, "funnel"):
		return domain.ProblemTailscaleFunnel
	}
	return domain.ProblemExposureFailed
}

type srcKey struct{}

// connContext keeps the internet client's address of a Funnel connection;
// its TCP peer is Tailscale's relay.
func connContext(ctx context.Context, c net.Conn) context.Context {
	if tc, ok := c.(*tls.Conn); ok {
		c = tc.NetConn()
	}
	if fc, ok := c.(*ipn.FunnelConn); ok {
		return context.WithValue(ctx, srcKey{}, fc.Src.Addr())
	}
	return ctx
}

// clientIP is the Funnel client's address, or the tailnet peer's.
func clientIP(r *http.Request) netip.Addr {
	if src, ok := r.Context().Value(srcKey{}).(netip.Addr); ok {
		return src.Unmap()
	}
	return ingress.RemoteAddr(r)
}
