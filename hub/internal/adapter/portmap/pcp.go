package portmap

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"time"
)

// PCP (RFC 6887) MAP requests: a 24-byte common header and the 36-byte MAP
// payload.
const (
	pcpVersion   = 2
	pcpOpMap     = 1
	pcpMapLen    = 60
	pcpProtoUDP  = 17
	pcpProtoTCP  = 6
	pcpLifetime  = 2 * time.Hour
	pcpResultOK  = 0
	pcpRespFlag  = 0x80
	pcpUnsupport = 1
)

// errPCPRefused is a PCP router saying no, as opposed to one not speaking PCP.
var errPCPRefused = errors.New("router refused")

func mapPCP(ctx context.Context, gw, self netip.Addr, nonce [12]byte, p Ports) (Mapping, func(context.Context), error) {
	if !self.IsValid() {
		return Mapping{}, nil, errors.New("no local address")
	}
	ext, life, err := pcpMap(ctx, gw, self, nonce, pcpProtoUDP, p.UDP, pcpLifetime)
	if err != nil {
		return Mapping{}, nil, err
	}
	tcp := p.TCP > 0
	if tcp {
		if _, _, err := pcpMap(ctx, gw, self, nonce, pcpProtoTCP, p.TCP, pcpLifetime); err != nil {
			tcp = false
		}
	}
	unmap := func(ctx context.Context) {
		_, _, _ = pcpMap(ctx, gw, self, nonce, pcpProtoUDP, p.UDP, 0)
		if tcp {
			_, _, _ = pcpMap(ctx, gw, self, nonce, pcpProtoTCP, p.TCP, 0)
		}
	}
	return Mapping{Protocol: "pcp", External: ext, Lifetime: life, TCP: tcp}, unmap, nil
}

// pcpMap asks for port on the same outside port; a zero lifetime deletes the
// mapping. The router may pick another port, which is undone and refused.
func pcpMap(ctx context.Context, gw, self netip.Addr, nonce [12]byte, proto byte, port int, life time.Duration) (netip.AddrPort, time.Duration, error) {
	req := make([]byte, pcpMapLen)
	req[0], req[1] = pcpVersion, pcpOpMap
	binary.BigEndian.PutUint32(req[4:8], uint32(life/time.Second)) //nolint:gosec // leases are hours at most
	client := self.As16()
	copy(req[8:24], client[:])
	copy(req[24:36], nonce[:])
	req[36] = proto
	binary.BigEndian.PutUint16(req[40:42], uint16(port)) //nolint:gosec // a port number
	if life > 0 {
		binary.BigEndian.PutUint16(req[42:44], uint16(port)) //nolint:gosec // a port number
	}
	// ::ffff:0.0.0.0 asks for any outside IPv4 address.
	anyV4 := netip.AddrFrom4([4]byte{}).As16()
	copy(req[44:60], anyV4[:])

	resp, err := exchange(ctx, gw, req)
	if err != nil {
		return netip.AddrPort{}, 0, err
	}
	if len(resp) < 2 || resp[0] != pcpVersion {
		// NAT-PMP routers answer an unknown version with their own header.
		return netip.AddrPort{}, 0, errors.New("router speaks NAT-PMP only")
	}
	if len(resp) < pcpMapLen || resp[1] != pcpRespFlag|pcpOpMap {
		return netip.AddrPort{}, 0, errors.New("malformed answer")
	}
	switch code := resp[3]; code {
	case pcpResultOK:
	case pcpUnsupport:
		return netip.AddrPort{}, 0, errors.New("router speaks another PCP version")
	default:
		return netip.AddrPort{}, 0, fmt.Errorf("%w (code %d)", errPCPRefused, code)
	}
	if life == 0 {
		return netip.AddrPort{}, 0, nil
	}
	got := binary.BigEndian.Uint16(resp[42:44])
	addr := netip.AddrFrom16([16]byte(resp[44:60])).Unmap()
	if int(got) != port {
		_, _, _ = pcpMap(ctx, gw, self, nonce, proto, port, 0)
		return netip.AddrPort{}, 0, fmt.Errorf("%w: offered another outside port", errPCPRefused)
	}
	return netip.AddrPortFrom(addr, got), time.Duration(binary.BigEndian.Uint32(resp[4:8])) * time.Second, nil
}
