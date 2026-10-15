// Package clientip decides which address a request came from.
package clientip

import (
	"net/http"
	"net/netip"
)

// Resolver returns the client address of a request. Each ingress supplies
// its own: forwarded headers are only trustworthy from a known proxy.
type Resolver func(r *http.Request) netip.Addr

// RemoteAddr trusts the TCP peer, which is right for direct connections.
func RemoteAddr(r *http.Request) netip.Addr {
	ap, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}
	}
	return ap.Addr().Unmap()
}
