// Package portmap asks the home router to forward the media server's ports,
// over NAT-PMP or UPnP-IGD. The outside port must equal the inside one: the
// media server advertises its own port with the public address it learns
// over STUN, so a mapping to any other port would be useless.
package portmap

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/huin/goupnp/dcps/internetgateway2"
	"tailscale.com/net/netmon"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

const (
	pmpPort      = 5351
	pmpTimeout   = 2 * time.Second
	description  = "Dislackso media"
	upnpLifetime = time.Hour
	pmpLifetime  = 2 * time.Hour
)

// Ports to forward, both on the same number outside.
type Ports struct {
	UDP int
	TCP int
}

// Mapping is what the router agreed to.
type Mapping = ports.MappedPorts

var ErrNoRouter = errors.New("no router answered NAT-PMP or UPnP")

// Mapper creates and removes the mappings.
type Mapper struct {
	mapping *Mapping
	unmap   func(ctx context.Context)
}

// Map forwards the ports, trying NAT-PMP first (one round trip) and then UPnP.
func (m *Mapper) Map(ctx context.Context, udp, tcp int) (Mapping, error) {
	p := Ports{UDP: udp, TCP: tcp}
	gw, self, ok := netmon.LikelyHomeRouterIP()
	var errs []error
	if ok && gw.Is4() {
		mp, err := mapPMP(ctx, gw, p)
		if err == nil {
			m.mapping = &mp
			m.unmap = func(ctx context.Context) { _ = unmapPMP(ctx, gw, p) }
			return mp, nil
		}
		errs = append(errs, fmt.Errorf("nat-pmp: %w", err))
	}
	mp, unmap, err := mapUPnP(ctx, self, p)
	if err == nil {
		m.mapping = &mp
		m.unmap = unmap
		return mp, nil
	}
	errs = append(errs, fmt.Errorf("upnp: %w", err))
	return Mapping{}, errors.Join(append([]error{ErrNoRouter}, errs...)...)
}

// Unmap removes what Map created.
func (m *Mapper) Unmap(ctx context.Context) {
	if m.unmap != nil {
		m.unmap(ctx)
	}
	m.mapping, m.unmap = nil, nil
}

func mapPMP(ctx context.Context, gw netip.Addr, p Ports) (Mapping, error) {
	ext, err := pmpExternal(ctx, gw)
	if err != nil {
		return Mapping{}, err
	}
	life, err := pmpMap(ctx, gw, 1, p.UDP, pmpLifetime)
	if err != nil {
		return Mapping{}, err
	}
	tcp := p.TCP > 0
	if tcp {
		if _, err := pmpMap(ctx, gw, 2, p.TCP, pmpLifetime); err != nil {
			tcp = false
		}
	}
	return Mapping{Protocol: "nat-pmp", External: netip.AddrPortFrom(ext, uint16(p.UDP)), Lifetime: life, TCP: tcp}, nil //nolint:gosec // a port number
}

func unmapPMP(ctx context.Context, gw netip.Addr, p Ports) error {
	_, err := pmpMap(ctx, gw, 1, p.UDP, 0)
	if p.TCP > 0 {
		_, _ = pmpMap(ctx, gw, 2, p.TCP, 0)
	}
	return err
}

// pmpRequest sends one NAT-PMP request and reads the answer (RFC 6886).
func pmpRequest(ctx context.Context, gw netip.Addr, req []byte, respLen int) ([]byte, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp4", netip.AddrPortFrom(gw, pmpPort).String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	deadline := time.Now().Add(pmpTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)
	if _, err := conn.Write(req); err != nil {
		return nil, err
	}
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	if n < respLen || buf[0] != 0 || buf[1] != req[1]+128 {
		return nil, errors.New("malformed answer")
	}
	if code := binary.BigEndian.Uint16(buf[2:4]); code != 0 {
		return nil, fmt.Errorf("router refused (code %d)", code)
	}
	return buf[:n], nil
}

func pmpExternal(ctx context.Context, gw netip.Addr) (netip.Addr, error) {
	resp, err := pmpRequest(ctx, gw, []byte{0, 0}, 12)
	if err != nil {
		return netip.Addr{}, err
	}
	return netip.AddrFrom4([4]byte(resp[8:12])), nil
}

// pmpMap asks for port on the same outside port; op 1 is UDP, 2 is TCP. A
// zero lifetime removes the mapping.
func pmpMap(ctx context.Context, gw netip.Addr, op byte, port int, life time.Duration) (time.Duration, error) {
	req := make([]byte, 12)
	req[1] = op
	binary.BigEndian.PutUint16(req[4:], uint16(port)) //nolint:gosec // a port number
	if life > 0 {
		binary.BigEndian.PutUint16(req[6:], uint16(port)) //nolint:gosec // a port number
	}
	binary.BigEndian.PutUint32(req[8:], uint32(life/time.Second)) //nolint:gosec // leases are an hour at most
	resp, err := pmpRequest(ctx, gw, req, 16)
	if err != nil {
		return 0, err
	}
	if life > 0 && int(binary.BigEndian.Uint16(resp[10:12])) != port {
		_, _ = pmpMap(ctx, gw, op, port, 0)
		return 0, errors.New("router offered another outside port")
	}
	return time.Duration(binary.BigEndian.Uint32(resp[12:16])) * time.Second, nil
}

// igd is what the three UPnP connection services have in common.
type igd interface {
	AddPortMappingCtx(ctx context.Context, remoteHost string, extPort uint16, proto string, intPort uint16,
		intClient string, enabled bool, desc string, lease uint32) error
	DeletePortMappingCtx(ctx context.Context, remoteHost string, extPort uint16, proto string) error
	GetExternalIPAddressCtx(ctx context.Context) (string, error)
}

func igdClients(ctx context.Context) []igd {
	var out []igd
	if cs, _, err := internetgateway2.NewWANIPConnection2ClientsCtx(ctx); err == nil {
		for _, c := range cs {
			out = append(out, c)
		}
	}
	if cs, _, err := internetgateway2.NewWANIPConnection1ClientsCtx(ctx); err == nil {
		for _, c := range cs {
			out = append(out, c)
		}
	}
	if cs, _, err := internetgateway2.NewWANPPPConnection1ClientsCtx(ctx); err == nil {
		for _, c := range cs {
			out = append(out, c)
		}
	}
	return out
}

func mapUPnP(ctx context.Context, self netip.Addr, p Ports) (Mapping, func(context.Context), error) {
	if !self.IsValid() {
		return Mapping{}, nil, errors.New("no local address")
	}
	clients := igdClients(ctx)
	if len(clients) == 0 {
		return Mapping{}, nil, errors.New("no gateway found")
	}
	var last error
	for _, c := range clients {
		extIP, err := c.GetExternalIPAddressCtx(ctx)
		if err != nil {
			last = err
			continue
		}
		ext, err := netip.ParseAddr(extIP)
		if err != nil {
			last = err
			continue
		}
		life := upnpLifetime
		if err := addUPnP(ctx, c, self, "UDP", p.UDP, life); err != nil {
			// Some routers only accept permanent mappings.
			life = 0
			if err := addUPnP(ctx, c, self, "UDP", p.UDP, life); err != nil {
				last = err
				continue
			}
		}
		tcp := p.TCP > 0 && addUPnP(ctx, c, self, "TCP", p.TCP, life) == nil
		unmap := func(ctx context.Context) {
			_ = c.DeletePortMappingCtx(ctx, "", uint16(p.UDP), "UDP") //nolint:gosec // a port number
			if tcp {
				_ = c.DeletePortMappingCtx(ctx, "", uint16(p.TCP), "TCP") //nolint:gosec // a port number
			}
		}
		return Mapping{Protocol: "upnp", External: netip.AddrPortFrom(ext, uint16(p.UDP)), Lifetime: life, TCP: tcp}, unmap, nil //nolint:gosec // a port number
	}
	return Mapping{}, nil, last
}

func addUPnP(ctx context.Context, c igd, self netip.Addr, proto string, port int, life time.Duration) error {
	return c.AddPortMappingCtx(ctx, "", uint16(port), proto, uint16(port), self.String(), true, description, //nolint:gosec // a port number
		uint32(life/time.Second)) //nolint:gosec // leases are an hour at most
}
