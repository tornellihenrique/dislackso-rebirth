package portmap

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"
)

var outside = netip.MustParseAddr("203.0.113.9")

// router answers PCP (when it speaks it) and NAT-PMP like a home router.
type router struct {
	pcp bool
	// otherPort makes the router offer a port other than the one asked for.
	otherPort bool

	mu     sync.Mutex
	mapped map[string]bool
}

func (r *router) serve(t *testing.T) netip.Addr {
	t.Helper()
	var lc net.ListenConfig
	conn, err := lc.ListenPacket(t.Context(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	ap := netip.MustParseAddrPort(conn.LocalAddr().String())
	saved := routerPort
	routerPort = ap.Port()
	t.Cleanup(func() { routerPort = saved })
	r.mapped = map[string]bool{}
	go func() {
		buf := make([]byte, 1100)
		for {
			n, from, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			if resp := r.answer(buf[:n]); resp != nil {
				_, _ = conn.WriteTo(resp, from)
			}
		}
	}()
	return ap.Addr()
}

func (r *router) answer(req []byte) []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case req[0] == pcpVersion && !r.pcp:
		return []byte{0, req[1] + 128, 0, 1, 0, 0, 0, 0}
	case req[0] == pcpVersion:
		resp := make([]byte, pcpMapLen)
		resp[0], resp[1] = pcpVersion, pcpRespFlag|pcpOpMap
		copy(resp[4:8], req[4:8])
		copy(resp[24:44], req[24:44])
		if r.otherPort {
			binary.BigEndian.PutUint16(resp[42:44], 40001)
		}
		ext := outside.As16()
		copy(resp[44:60], ext[:])
		key := "pcp/" + string(rune(req[36])) + "/" + string(req[40:42])
		r.mapped[key] = binary.BigEndian.Uint32(req[4:8]) > 0
		return resp
	case req[1] == 0:
		ip := outside.As4()
		return append([]byte{0, 128, 0, 0, 0, 0, 0, 0}, ip[:]...)
	default:
		resp := make([]byte, 16)
		resp[1] = req[1] + 128
		copy(resp[8:10], req[4:6])
		copy(resp[10:12], req[6:8])
		if r.otherPort && binary.BigEndian.Uint32(req[8:12]) > 0 {
			binary.BigEndian.PutUint16(resp[10:12], 40001)
		}
		copy(resp[12:16], req[8:12])
		key := "pmp/" + string(rune(req[1])) + "/" + string(req[4:6])
		r.mapped[key] = binary.BigEndian.Uint32(req[8:12]) > 0
		return resp
	}
}

func (r *router) active() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, on := range r.mapped {
		if on {
			n++
		}
	}
	return n
}

func TestMapGateway(t *testing.T) {
	self := netip.MustParseAddr("127.0.0.1")
	tests := []struct {
		name           string
		pcp, otherPort bool
		protocol       string
		err            bool
	}{
		{name: "PCP router", pcp: true, protocol: "pcp"},
		{name: "NAT-PMP router", protocol: "nat-pmp"},
		{name: "PCP router with another port", pcp: true, otherPort: true, err: true},
		{name: "NAT-PMP router with another port", otherPort: true, err: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &router{pcp: tt.pcp, otherPort: tt.otherPort}
			gw := r.serve(t)
			var m Mapper
			got, err := m.mapGateway(t.Context(), gw, self, Ports{UDP: 7882, TCP: 7881})
			if tt.err {
				if err == nil {
					t.Fatalf("mapping = %+v, want an error", got)
				}
				if r.active() != 0 {
					t.Error("a mapping on the wrong port was left on the router")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := netip.AddrPortFrom(outside, 7882)
			if got.Protocol != tt.protocol || got.External != want || !got.TCP || got.Lifetime <= 0 {
				t.Errorf("mapping = %+v, want %s at %s with TCP", got, tt.protocol, want)
			}
			if r.active() != 2 {
				t.Errorf("router holds %d mappings, want UDP and TCP", r.active())
			}
			m.Unmap(t.Context())
			if r.active() != 0 {
				t.Errorf("router holds %d mappings after Unmap", r.active())
			}
		})
	}
}

func TestMapGatewayGivesUpWithoutAnAnswer(t *testing.T) {
	var lc net.ListenConfig
	silent, err := lc.ListenPacket(t.Context(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = silent.Close() }()
	saved := routerPort
	routerPort = netip.MustParseAddrPort(silent.LocalAddr().String()).Port()
	defer func() { routerPort = saved }()

	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	var m Mapper
	_, err = m.mapGateway(ctx, netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("127.0.0.1"), Ports{UDP: 7882})
	if err == nil || errors.Is(err, errPCPRefused) {
		t.Errorf("err = %v, want both protocols unanswered", err)
	}
}
