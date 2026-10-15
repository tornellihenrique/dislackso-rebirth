// Package httpauth holds the request checks shared by every cookie-
// authenticated HTTP entry point: RPC, the gateway and file routes.
package httpauth

import (
	"net/http"
	"net/url"
	"strings"
)

// SessionCookie holds the opaque session token.
const SessionCookie = "dsx_session"

// SameOrigin refuses requests whose Origin names another site. Browsers send
// Origin on cross-site and state-changing requests and on every WebSocket
// handshake; tools that send none cannot ride the user's cookies anyway.
func SameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// SessionToken returns the raw session token from the cookie, if any.
func SessionToken(r *http.Request) string {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}
