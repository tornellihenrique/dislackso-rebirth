package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/ddns"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/exposure/cloudflared"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/exposure/direct"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/exposure/tailscale"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/hostos"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/ice"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/publicprobe"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/stun"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/exposure"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/relay"
)

const (
	relayAPITimeout = 10 * time.Second
	dnsAPITimeout   = 15 * time.Second
)

// exposureProviders builds the provider for the exposure settings; state
// each provider keeps (tailnet login, certificates, cloudflared) lives in
// the data dir.
func exposureProviders(dataDir string, clk ports.Clock, logger *slog.Logger) exposure.Factory {
	dnsClient := &http.Client{Timeout: dnsAPITimeout}
	return func(s domain.ExposureSettings, secrets exposure.Secrets) ports.ExposureProvider {
		switch s.Kind {
		case domain.ExposureTailscaleFunnel:
			return &tailscale.Provider{
				Dir: filepath.Join(dataDir, "tsnet"), Hostname: s.TailscaleHostname,
				Probe: publicprobe.Prober{}, Logger: logger, Clock: clk,
			}
		case domain.ExposureCloudflareQuick, domain.ExposureCloudflareTunnel:
			return &cloudflared.Provider{
				Quick:     s.Kind == domain.ExposureCloudflareQuick,
				Token:     secrets.TunnelToken,
				TunnelURL: s.TunnelURL,
				Installer: cloudflared.Installer{Dir: filepath.Join(dataDir, "sidecars")},
				LogPath:   filepath.Join(dataDir, "logs", "cloudflared.log"),
				Logger:    logger,
				Clock:     clk,
			}
		case domain.ExposureDirect:
			p := &direct.Public{
				Domain: s.DirectDomain, Email: s.DirectEmail, Addr: s.DirectAddr,
				CacheDir: filepath.Join(dataDir, "acme"), Logger: logger, Clock: clk,
			}
			if s.DynamicDNS == domain.DynamicDNSDuckDNS {
				p.DNS = ddns.DuckDNS{Token: secrets.DNSToken, Client: dnsClient}
				p.PublicIP = publicIPv4
			}
			return p
		case domain.ExposureLAN:
			return &direct.LAN{Addr: s.LANAddr, CertDir: filepath.Join(dataDir, "tls"), Logger: logger, Clock: clk}
		case domain.ExposureLocal:
		}
		return nil
	}
}

// relayProviders builds the TURN provider for the relay settings.
func relayProviders(clk ports.Clock) relay.Factory {
	client := &http.Client{Timeout: relayAPITimeout}
	return func(s domain.TURNSettings, secret string) ports.IceServerProvider {
		switch s.Kind {
		case domain.TURNStatic:
			return ice.Static{URLs: s.URLs, Username: s.Username, Credential: secret}
		case domain.TURNCoturnREST:
			return ice.CoturnREST{URLs: s.URLs, Secret: secret, Now: clk.Now}
		case domain.TURNCloudflare:
			return ice.Cloudflare{KeyID: s.CloudflareKeyID, APIToken: secret, Client: client}
		case domain.TURNNone:
		}
		return nil
	}
}

func stunProbe(ctx context.Context, conn net.PacketConn, network string) []netip.AddrPort {
	return stun.Mapped(ctx, conn, network, stun.Servers)
}

// publicIPv4 is this machine's public IPv4 as STUN servers see it.
func publicIPv4(ctx context.Context) (netip.Addr, error) {
	var lc net.ListenConfig
	conn, err := lc.ListenPacket(ctx, "udp4", ":0")
	if err != nil {
		return netip.Addr{}, err
	}
	defer func() { _ = conn.Close() }()
	for _, m := range stunProbe(ctx, conn, "udp4") {
		if m.IsValid() {
			return m.Addr(), nil
		}
	}
	return netip.Addr{}, errors.New("no STUN server answered")
}

// hostMachine is this machine for the host service: run at login repeats
// how the hub was started this time.
func hostMachine(media string) *hostos.Host {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	return &hostos.Host{Launch: hostos.Launch{Exe: exe, Args: os.Args[1:]}, Media: media}
}
