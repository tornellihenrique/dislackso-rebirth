// Package clientip decides which address a request came from.
package clientip

import (
	"net/http"
	"net/netip"
)

// Resolver returns the client address of a request. Each ingress supplies
// its own: forwarded headers are only trustworthy from a known proxy.
type Resolver func(r *http.Request) netip.Addr
