// Package ingress tells handlers how a request reached the hub: through its
// own listener on this machine, or through an exposure provider (a tunnel,
// a public or LAN listener). Each ingress decides whom to trust for the
// client address and whether the client used HTTPS.
package ingress

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"time"
)

// Info describes one way into the hub.
type Info struct {
	// Name is the exposure provider, or "local" for the hub's own listener.
	Name string
	// Local is true only for the hub's own listener, the one place requests
	// from this machine (setup, media webhooks) can come from.
	Local bool
	// Secure means the client connected over HTTPS, even when TLS ended
	// before the hub (a tunnel).
	Secure bool
	// HSTS asks browsers to keep using HTTPS; only for certificates they trust.
	HSTS bool
	// ClientIP extracts the client address; nil uses the TCP peer.
	ClientIP func(r *http.Request) netip.Addr
	// ConnContext adds per-connection values, such as a tunnel's client address.
	ConnContext func(ctx context.Context, c net.Conn) context.Context
}

// Local is the hub's own listener: clients connect straight to it.
var Local = Info{Name: "local", Local: true}

type infoKey struct{}

// From returns the ingress a request came through; requests from handlers
// not served by Serve (tests) count as local.
func From(r *http.Request) Info {
	if info, ok := r.Context().Value(infoKey{}).(Info); ok {
		return info
	}
	return Local
}

// ClientIP is the trusted client address. Only the local ingress may yield a
// loopback address, so a tunnel's own hop is never taken for this machine.
func ClientIP(r *http.Request) netip.Addr {
	info := From(r)
	ip := RemoteAddr(r)
	if info.ClientIP != nil {
		ip = info.ClientIP(r)
	}
	if !info.Local && ip.IsLoopback() {
		return netip.Addr{}
	}
	return ip
}

// IsLocal reports a request from this machine through the hub's own listener.
func IsLocal(r *http.Request) bool {
	return From(r).Local && RemoteAddr(r).IsLoopback()
}

// Secure reports whether the client used HTTPS.
func Secure(r *http.Request) bool { return r.TLS != nil || From(r).Secure }

// RemoteAddr is the TCP peer.
func RemoteAddr(r *http.Request) netip.Addr {
	ap, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}
	}
	return ap.Addr().Unmap()
}

const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 2 * time.Minute
)

// Server serves one listener for one ingress.
type Server struct {
	srv *http.Server
	ln  net.Listener
}

// NewServer prepares handler on ln, tagging every request with info.
func NewServer(ctx context.Context, ln net.Listener, handler http.Handler, info Info, logger *slog.Logger) *Server {
	base := context.WithValue(context.WithoutCancel(ctx), infoKey{}, info)
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
		BaseContext:       func(net.Listener) context.Context { return base },
		ConnContext:       info.ConnContext,
	}
	return &Server{srv: srv, ln: ln}
}

// Serve blocks until Shutdown or a listener error; a shutdown returns nil.
func (s *Server) Serve() error {
	if err := s.srv.Serve(s.ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown stops accepting and waits for requests in flight.
func (s *Server) Shutdown(ctx context.Context) error { return s.srv.Shutdown(ctx) }

// Close stops at once.
func (s *Server) Close() error { return s.srv.Close() }

// Addr is where the server listens.
func (s *Server) Addr() net.Addr { return s.ln.Addr() }

// Serve runs handler on ln until ctx ends, then drains for up to grace.
func Serve(ctx context.Context, ln net.Listener, handler http.Handler, info Info, logger *slog.Logger, grace time.Duration) error {
	s := NewServer(ctx, ln, handler, info, logger)
	done := make(chan error, 1)
	go func() { done <- s.Serve() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), grace)
	defer cancel()
	if err := s.Shutdown(shutdownCtx); err != nil {
		_ = s.Close()
	}
	return <-done
}
