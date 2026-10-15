package rpc

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect/v2"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/locale"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/auth"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/clientip"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/httpauth"
)

// SessionCookie holds the opaque session token.
const SessionCookie = httpauth.SessionCookie

type Authenticator interface {
	Authenticate(ctx context.Context, raw string) (auth.Principal, error)
}

type requestInfo struct {
	client    domain.ClientInfo
	principal *auth.Principal
	secure    bool
}

type requestKey struct{}

func withRequest(ctx context.Context, info *requestInfo) context.Context {
	return context.WithValue(ctx, requestKey{}, info)
}

func requestFrom(ctx context.Context) *requestInfo {
	if info, ok := ctx.Value(requestKey{}).(*requestInfo); ok {
		return info
	}
	return &requestInfo{}
}

func clientFrom(ctx context.Context) domain.ClientInfo { return requestFrom(ctx).client }

// requireActor returns the signed-in caller or an Unauthenticated error.
func requireActor(ctx context.Context) (domain.Actor, error) {
	if p := requestFrom(ctx).principal; p != nil {
		return p.Actor, nil
	}
	return domain.Actor{}, connect.NewError(connect.CodeUnauthenticated, "sign in required").
		WithDetail(reasonDetail(domain.ReasonSessionExpired))
}

// middleware runs before every RPC: it rejects cross-site requests, records
// who is calling, and resolves the session cookie.
func middleware(next http.Handler, authn Authenticator, resolveIP clientip.Resolver, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !httpauth.SameOrigin(r) {
			http.Error(w, "cross-origin request refused", http.StatusForbidden)
			return
		}
		info := &requestInfo{
			client: domain.ClientInfo{
				IP:        resolveIP(r),
				UserAgent: r.UserAgent(),
				Locale:    locale.Match(r.Header.Get("Accept-Language")),
			},
			secure: r.TLS != nil,
		}
		if c, err := r.Cookie(SessionCookie); err == nil && c.Value != "" {
			p, err := authn.Authenticate(r.Context(), c.Value)
			switch {
			case err == nil:
				info.principal = &p
				if p.Refreshed {
					http.SetCookie(w, sessionCookie(c.Value, p.Session.ExpiresAt, info.secure))
				}
			case errors.Is(err, domain.ErrUnauthenticated):
				http.SetCookie(w, clearedCookie(info.secure))
			default:
				logger.ErrorContext(r.Context(), "authenticate session", slog.Any("error", err))
			}
		}
		next.ServeHTTP(w, r.WithContext(withRequest(r.Context(), info)))
	})
}

// Secure follows the request scheme so the hub also works over plain HTTP
// on localhost.
func sessionCookie(token string, expires time.Time, secure bool) *http.Cookie {
	return &http.Cookie{ //nolint:gosec // Secure is set whenever the request used TLS
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func clearedCookie(secure bool) *http.Cookie {
	return &http.Cookie{ //nolint:gosec // Secure is set whenever the request used TLS
		Name:     SessionCookie,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// setCookie adds a Set-Cookie header to the RPC response.
func setCookie(ctx context.Context, c *http.Cookie) {
	if info, ok := connect.CallInfoForServerContext(ctx); ok {
		info.ResponseHeader().Add("Set-Cookie", c.String())
	}
}

func startSession(ctx context.Context, signed auth.SignedIn) {
	setCookie(ctx, sessionCookie(signed.Token, signed.Session.ExpiresAt, requestFrom(ctx).secure))
}

func endSession(ctx context.Context) {
	setCookie(ctx, clearedCookie(requestFrom(ctx).secure))
}
