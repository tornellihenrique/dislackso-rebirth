// Package stun asks STUN servers for the public address of a UDP socket.
package stun

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"time"

	"github.com/pion/stun/v3"
)

// Servers are two independent providers, so a symmetric NAT shows up as two
// different outside ports.
var Servers = []string{"stun.cloudflare.com:3478", "stun.l.google.com:19302"}

const answerTimeout = 1500 * time.Millisecond

// Mapped asks each server for conn's public address. A server that does not
// answer in time yields an invalid address in its place.
func Mapped(ctx context.Context, conn net.PacketConn, network string, servers []string) []netip.AddrPort {
	out := make([]netip.AddrPort, len(servers))
	for i, server := range servers {
		if ctx.Err() != nil {
			break
		}
		if ap, err := binding(ctx, conn, network, server); err == nil {
			out[i] = ap
		}
	}
	return out
}

func binding(ctx context.Context, conn net.PacketConn, network, server string) (netip.AddrPort, error) {
	var r net.Resolver
	host, port, err := net.SplitHostPort(server)
	if err != nil {
		return netip.AddrPort{}, err
	}
	// "udp4" looks up "ip4" addresses.
	ips, err := r.LookupNetIP(ctx, "ip"+network[len("udp"):], host)
	if err != nil || len(ips) == 0 {
		return netip.AddrPort{}, errors.New("stun server not found")
	}
	addr, err := net.ResolveUDPAddr(network, net.JoinHostPort(ips[0].String(), port))
	if err != nil {
		return netip.AddrPort{}, err
	}
	req := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	if _, err := conn.WriteTo(req.Raw, addr); err != nil {
		return netip.AddrPort{}, err
	}
	deadline := time.Now().Add(answerTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetReadDeadline(deadline)
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()
	buf := make([]byte, 1500)
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			return netip.AddrPort{}, err
		}
		res := &stun.Message{Raw: append([]byte(nil), buf[:n]...)}
		if res.Decode() != nil || res.TransactionID != req.TransactionID {
			continue
		}
		var xa stun.XORMappedAddress
		if err := xa.GetFrom(res); err != nil {
			return netip.AddrPort{}, err
		}
		ip, ok := netip.AddrFromSlice(xa.IP)
		if !ok {
			return netip.AddrPort{}, errors.New("bad mapped address")
		}
		return netip.AddrPortFrom(ip.Unmap(), uint16(xa.Port)), nil //nolint:gosec // a port number
	}
}

// LocalAddrs lists this machine's interface addresses, to tell a public
// address of its own from one a router gave it.
func LocalAddrs() []netip.Addr {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	out := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		if p, err := netip.ParsePrefix(a.String()); err == nil {
			out = append(out, p.Addr().Unmap())
		}
	}
	return out
}
