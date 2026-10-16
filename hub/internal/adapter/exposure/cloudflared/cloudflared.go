// Package cloudflared publishes the hub through a Cloudflare tunnel run by a
// cloudflared sidecar: a quick tunnel (random trycloudflare.com address, no
// account) or a named tunnel to the team's own domain.
package cloudflared

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/sidecar"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/ingress"
)

// TunnelPort is where a named tunnel's public hostname must point in the
// Cloudflare dashboard ("http://localhost:7301"): its routes are set there,
// not by the hub.
const TunnelPort = 7301

const (
	drainTimeout = 5 * time.Second
	readyTimeout = 2 * time.Second
)

var quickURLRe = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

type Provider struct {
	// Quick runs a quick tunnel; otherwise a named one with Token.
	Quick     bool
	Token     string
	TunnelURL string
	Installer Installer
	LogPath   string
	Logger    *slog.Logger
	Clock     ports.Clock
}

func (p *Provider) Kind() domain.ExposureKind {
	if p.Quick {
		return domain.ExposureCloudflareQuick
	}
	return domain.ExposureCloudflareTunnel
}

func (p *Provider) status(state domain.ExposureState) domain.ExposureStatus {
	return domain.ExposureStatus{Kind: p.Kind(), State: state, Since: p.Clock.Now()}
}

func (p *Provider) Run(ctx context.Context, handler http.Handler, report func(domain.ExposureStatus)) error {
	report(p.status(domain.ExposureStarting))
	if !p.Quick && p.Token == "" {
		return &ports.ExposureError{Problem: domain.ProblemTunnelToken, Err: errors.New("no tunnel token")}
	}
	bin, err := p.Installer.Ensure(ctx)
	if err != nil {
		return &ports.ExposureError{Problem: domain.ProblemCloudflaredFetch, Err: err}
	}
	// A listener of its own: what arrives here came through the tunnel,
	// even though cloudflared connects from this machine.
	addr := "127.0.0.1:0"
	if !p.Quick {
		addr = "127.0.0.1:" + strconv.Itoa(TunnelPort)
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return &ports.ExposureError{Problem: domain.ProblemAddressInUse, Err: err}
	}
	metrics, err := freePort(ctx)
	if err != nil {
		_ = ln.Close()
		return &ports.ExposureError{Problem: domain.ProblemExposureFailed, Err: err}
	}
	info := ingress.Info{Name: string(p.Kind()), Secure: true, HSTS: true, ClientIP: clientIP}
	served := make(chan error, 1)
	go func() { served <- ingress.Serve(ctx, ln, handler, info, p.Logger, drainTimeout) }()

	metricsAddr := "127.0.0.1:" + strconv.Itoa(metrics)
	args := []string{"tunnel", "--no-autoupdate", "--metrics", metricsAddr}
	var env []string
	if p.Quick {
		args = append(args, "--url", "http://"+ln.Addr().String())
	} else {
		// The token travels in the environment, out of the process list.
		args = append(args, "run")
		env = append(env, "TUNNEL_TOKEN="+p.Token)
	}
	var mu sync.Mutex
	url := p.TunnelURL
	sup := sidecar.New(sidecar.Spec{
		Name:    "cloudflared",
		Path:    bin,
		Args:    args,
		Env:     env,
		LogPath: p.LogPath,
		Health:  ready(metricsAddr),
		OnOutput: func(line string) {
			if !p.Quick {
				return
			}
			if found := quickURLRe.FindString(line); found != "" {
				mu.Lock()
				url = found
				mu.Unlock()
			}
		},
		OnReady: func(context.Context, int) {
			mu.Lock()
			s := p.status(domain.ExposureRunning)
			s.URL = url
			mu.Unlock()
			report(s)
		},
	}, p.Logger)
	runErr := sup.Run(ctx)
	if err := <-served; err != nil && ctx.Err() == nil {
		return &ports.ExposureError{Problem: domain.ProblemExposureFailed, Err: err}
	}
	if runErr != nil && ctx.Err() == nil {
		return &ports.ExposureError{Problem: domain.ProblemCloudflaredExited, Err: runErr}
	}
	return nil
}

// ready asks cloudflared's metrics server whether a tunnel connection is up.
func ready(addr string) func(ctx context.Context) error {
	client := &http.Client{Timeout: readyTimeout}
	return func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/ready", http.NoBody)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("cloudflared not connected: %s", resp.Status)
		}
		return nil
	}
}

func freePort(ctx context.Context) (int, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// clientIP takes Cloudflare's word for the client: only cloudflared on this
// machine connects to the tunnel's listener.
func clientIP(r *http.Request) netip.Addr {
	if ip, err := netip.ParseAddr(r.Header.Get("Cf-Connecting-Ip")); err == nil {
		return ip.Unmap()
	}
	return netip.Addr{}
}
