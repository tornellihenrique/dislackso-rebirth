package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/livekit"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/config"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/sidecar"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/media"
)

const (
	// Names of entries in the secret store.
	secretLiveKitKey    = "livekit.api_key"    //nolint:gosec // a key name, not a credential
	secretLiveKitSecret = "livekit.api_secret" //nolint:gosec // a key name, not a credential
	healthProbeTimeout  = 2 * time.Second
	// addressCheckEvery is how often the hub looks at its public address.
	addressCheckEvery = time.Minute
)

// mediaServer is the LiveKit sidecar: its keys, its supervisor once the hub
// listens, and the provider services use.
type mediaServer struct {
	cfg config.LiveKit
	// findNodeIP finds, before each start, the public IPv4 at which the router
	// keeps the media port's number; empty behind other routers. It stays nil
	// on hosts told not to look up their public address.
	findNodeIP func(ctx context.Context) string
	// publicIP is this host's public IPv4 now, wherever it listens.
	publicIP   func(ctx context.Context) (netip.Addr, error)
	checkEvery time.Duration
	// nodeIP is what LiveKit advertises since its last start.
	nodeIP   atomic.Pointer[string]
	dataDir  string
	logLevel string
	logger   *slog.Logger
	// binary is empty when voice is off or livekit-server was not found.
	binary   string
	key      string
	secret   string
	provider *livekit.Provider
	sup      atomic.Pointer[sidecar.Supervisor]
}

func newMediaServer(ctx context.Context, cfg config.Config, secrets ports.SecretStore, logger *slog.Logger) (*mediaServer, error) {
	m := &mediaServer{
		cfg: cfg.LiveKit, checkEvery: addressCheckEvery,
		dataDir: cfg.DataDir, logLevel: cfg.Log.Level, logger: logger,
	}
	key, err := ensureSecret(ctx, secrets, secretLiveKitKey, func() string { return "API" + rand.Text()[:12] })
	if err != nil {
		return nil, err
	}
	secret, err := ensureSecret(ctx, secrets, secretLiveKitSecret, func() string { return rand.Text() + rand.Text() })
	if err != nil {
		return nil, err
	}
	m.key, m.secret = key, secret
	m.provider = livekit.New(livekit.Config{
		PublicURL: media.ProxyPrefix,
		APIURL:    m.apiURL().String(),
		APIKey:    key,
		APISecret: secret,
		Ready:     m.Ready,
	})
	if !cfg.LiveKit.Disabled {
		if m.binary, err = findLiveKit(cfg.LiveKit.Binary); err != nil {
			logger.WarnContext(ctx, "voice is unavailable: livekit-server not found; set livekit.binary", slog.Any("error", err))
		}
	}
	return m, nil
}

func ensureSecret(ctx context.Context, s ports.SecretStore, key string, generate func() string) (string, error) {
	v, ok, err := s.Secret(ctx, key)
	if err != nil {
		return "", err
	}
	if ok && v != "" {
		return v, nil
	}
	v = generate()
	if err := s.SetSecret(ctx, key, v); err != nil {
		return "", err
	}
	return v, nil
}

// findLiveKit looks for the binary configured, then next to the hub
// executable (release archives ship both), then on PATH.
func findLiveKit(configured string) (string, error) {
	if configured != "" {
		if _, err := os.Stat(configured); err != nil {
			return "", err
		}
		return configured, nil
	}
	name := "livekit-server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return exec.LookPath(name)
}

func (m *mediaServer) apiURL() *url.URL {
	return &url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(m.cfg.Port))}
}

func (m *mediaServer) Ready() bool {
	s := m.sup.Load()
	return s != nil && s.Ready()
}

func (m *mediaServer) Name() string { return "media" }

// Check fails while the sidecar is configured but not serving; a hub
// without voice has nothing to check.
func (m *mediaServer) Check(context.Context) error {
	if m.binary == "" || m.Ready() {
		return nil
	}
	state := sidecar.StateStopped
	if s := m.sup.Load(); s != nil {
		state = s.Status().State
	}
	return fmt.Errorf("media server %s", state)
}

// start supervises the sidecar until ctx is done, writing livekit.yaml before
// each of its starts. hubAddr is where the hub listens, for the webhook URL.
func (m *mediaServer) start(ctx context.Context, wg *sync.WaitGroup, hubAddr net.Addr, onReady func(ctx context.Context, restarts int)) error {
	if m.binary == "" {
		return nil
	}
	tcp, ok := hubAddr.(*net.TCPAddr)
	if !ok {
		return errors.New("hub address is not TCP")
	}
	path := filepath.Join(m.dataDir, "livekit.yaml")
	writeConfig := func(ctx context.Context) error {
		nodeIP := ""
		if m.findNodeIP != nil {
			nodeIP = m.findNodeIP(ctx)
		}
		m.nodeIP.Store(&nodeIP)
		return livekit.WriteServerConfig(path, livekit.ServerOptions{
			Port: m.cfg.Port, UDPPort: m.cfg.UDPPort, TCPPort: m.cfg.TCPPort, APIKey: m.key,
			WebhookURL: "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(tcp.Port)) + media.WebhookPath,
			NodeIP:     nodeIP, LogLevel: m.logLevel,
		})
	}
	health := m.apiURL().String() + "/"
	sup := sidecar.New(sidecar.Spec{
		Name:        "livekit",
		Path:        m.binary,
		Args:        []string{"--config", path},
		Env:         []string{livekit.KeysEnv(m.key, m.secret)},
		LogPath:     filepath.Join(m.dataDir, "logs", "livekit.log"),
		BeforeStart: writeConfig,
		Health:      func(ctx context.Context) error { return probe(ctx, health) },
		OnReady:     onReady,
	}, m.logger)
	m.sup.Store(sup)
	wg.Go(func() {
		if err := sup.Run(ctx); err != nil {
			m.logger.ErrorContext(ctx, "voice is unavailable", slog.Any("error", err))
		}
	})
	if m.publicIP != nil {
		wg.Go(func() { m.followPublicAddress(ctx, sup) })
	}
	return nil
}

// followPublicAddress restarts the media server when the address it
// advertises stops being this host's, which sends every caller to a dead
// address: the provider changed it, or the hub started behind a VPN since
// turned off. Connections already up break with the old address anyway.
func (m *mediaServer) followPublicAddress(ctx context.Context, sup *sidecar.Supervisor) {
	var watch addressWatch
	tick := time.NewTicker(m.checkEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		advertised := m.nodeIP.Load()
		if advertised == nil {
			continue
		}
		seen := ""
		if ip, err := m.publicIP(ctx); err == nil {
			seen = ip.String()
		}
		if watch.moved(*advertised, seen) {
			m.logger.InfoContext(ctx, "the public address changed; restarting the media server",
				slog.String("advertised", *advertised), slog.String("public", seen))
			sup.Restart()
		}
	}
}

// addressWatch tells a moved public address from a passing odd answer: the
// same other address has to show twice in a row. Without an advertised
// address nothing can go stale, as LiveKit finds it per connection; and no
// answer (offline) says nothing about it.
type addressWatch struct{ pending string }

func (w *addressWatch) moved(advertised, seen string) bool {
	if advertised == "" || seen == "" || seen == advertised {
		w.pending = ""
		return false
	}
	if seen != w.pending {
		w.pending = seen
		return false
	}
	w.pending = ""
	return true
}

// probe passes once LiveKit answers its health check with 200.
func probe(ctx context.Context, rawURL string) error {
	ctx, cancel := context.WithTimeout(ctx, healthProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("health check: %s", res.Status)
	}
	return nil
}
