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
	certTimeout  = 3 * time.Minute
	// Tailscale publishes a new Funnel name in public DNS within about ten
	// minutes; past this the hub stops waiting and serves unverified.
	publishWait = 15 * time.Minute
	probeEvery  = 15 * time.Second
)

// Provider runs a tsnet node whose state lives in Dir, so it stays signed in
// across restarts.
type Provider struct {
	Dir      string
	Hostname string
	// Probe confirms the address works from the internet before it is
	// reported running; nil reports it at once.
	Probe  ports.PublicProbe
	Logger *slog.Logger
	Clock  ports.Clock
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
	url := "https://" + domains[0]
	info := ingress.Info{
		Name:        string(domain.ExposureTailscaleFunnel),
		Secure:      true,
		HSTS:        true,
		ClientIP:    clientIP,
		ConnContext: connContext,
	}
	serveCtx, stopServing := context.WithCancel(ctx)
	defer stopServing()
	served := make(chan error, 1)
	go func() { served <- ingress.Serve(serveCtx, ln, handler, info, p.Logger, drainTimeout) }()
	stop := func(err error) error {
		stopServing()
		<-served
		if ctx.Err() == nil {
			return err
		}
		return nil
	}

	// The certificate is otherwise issued during the first visitor's
	// handshake, which then waits about a minute.
	report(p.stage(domain.ExposureStarting, domain.StageCertificate, url))
	certCtx, cancel := context.WithTimeout(serveCtx, certTimeout)
	_, _, err = lc.CertPair(certCtx, domains[0])
	cancel()
	if err != nil {
		return stop(&ports.ExposureError{Problem: domain.ProblemCertificate, Err: err})
	}

	running := p.stage(domain.ExposureRunning, "", url)
	if p.Probe != nil {
		report(p.stage(domain.ExposureStarting, domain.StagePublishing, url))
		if !awaitPublished(serveCtx, p.Probe, url+"/", probeEvery, publishWait) {
			if ctx.Err() != nil {
				return stop(nil)
			}
			running.Stage = domain.StageUnverified
		}
	}
	report(running)
	if err := <-served; err != nil && ctx.Err() == nil {
		return &ports.ExposureError{Problem: domain.ProblemExposureFailed, Err: err}
	}
	return nil
}

func (p *Provider) stage(state domain.ExposureState, stage, url string) domain.ExposureStatus {
	s := p.status(state)
	s.Stage, s.URL = stage, url
	return s
}

// awaitPublished tries url every interval until it answers from the
// internet or limit passes, and reports whether it answered.
func awaitPublished(ctx context.Context, probe ports.PublicProbe, url string, interval, limit time.Duration) bool {
	deadline := time.Now().Add(limit)
	for {
		if probe.Reachable(ctx, url) == nil {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(interval):
		}
	}
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
