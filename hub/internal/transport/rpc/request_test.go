package rpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/proto"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/auth"
)

type stubAuthenticator struct {
	principals map[string]auth.Principal
	err        error
}

func (s stubAuthenticator) Authenticate(_ context.Context, raw string) (auth.Principal, error) {
	if s.err != nil {
		return auth.Principal{}, s.err
	}
	if p, ok := s.principals[raw]; ok {
		return p, nil
	}
	return auth.Principal{}, domain.Unauthenticated(domain.ReasonSessionExpired)
}

func TestMiddleware(t *testing.T) {
	expires := time.Unix(1_800_000_000, 0)
	authn := stubAuthenticator{principals: map[string]auth.Principal{
		"good":  {Actor: domain.Actor{UserID: 7}},
		"stale": {Actor: domain.Actor{UserID: 8}, Session: domain.Session{ExpiresAt: expires}, Refreshed: true},
	}}

	tests := []struct {
		name        string
		cookie      string
		origin      string
		wantStatus  int
		wantUser    domain.ID
		wantCookie  string
		wantNoCooke bool
	}{
		{name: "anonymous", wantStatus: http.StatusOK},
		{name: "valid session", cookie: "good", wantStatus: http.StatusOK, wantUser: 7, wantNoCooke: true},
		{name: "refreshed session renews the cookie", cookie: "stale", wantStatus: http.StatusOK, wantUser: 8, wantCookie: "dsx_session=stale"},
		{name: "expired session clears the cookie", cookie: "gone", wantStatus: http.StatusOK, wantCookie: "Max-Age=0"},
		{name: "same origin", origin: "http://hub.test", wantStatus: http.StatusOK},
		{name: "cross origin", origin: "https://evil.example", wantStatus: http.StatusForbidden},
		{name: "opaque origin", origin: "null", wantStatus: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser domain.ID
			var gotClient domain.ClientInfo
			next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				if actor, err := requireActor(r.Context()); err == nil {
					gotUser = actor.UserID
				}
				gotClient = clientFrom(r.Context())
			})
			h := middleware(next, authn, func(*http.Request) netip.Addr { return netip.MustParseAddr("192.0.2.1") }, slog.New(slog.DiscardHandler))

			req := httptest.NewRequest(http.MethodPost, "http://hub.test/rpc/x", http.NoBody)
			req.Header.Set("Accept-Language", "pt-BR")
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: SessionCookie, Value: tt.cookie})
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if gotUser != tt.wantUser {
				t.Errorf("user = %d, want %d", gotUser, tt.wantUser)
			}
			setCookie := rec.Header().Get("Set-Cookie")
			if tt.wantCookie != "" && !strings.Contains(setCookie, tt.wantCookie) {
				t.Errorf("Set-Cookie = %q, want %q", setCookie, tt.wantCookie)
			}
			if tt.wantNoCooke && setCookie != "" {
				t.Errorf("unexpected Set-Cookie %q", setCookie)
			}
			if tt.wantStatus == http.StatusOK && (gotClient.Locale != "pt-BR" || gotClient.IP.String() != "192.0.2.1") {
				t.Errorf("client = %+v", gotClient)
			}
		})
	}
}

func TestSessionCookieAttributes(t *testing.T) {
	c := sessionCookie("tok", time.Unix(1_800_000_000, 0), true).String()
	for _, want := range []string{"dsx_session=tok", "Path=/", "HttpOnly", "Secure", "SameSite=Lax", "Expires="} {
		if !strings.Contains(c, want) {
			t.Errorf("cookie %q lacks %q", c, want)
		}
	}
}

func TestToConnect(t *testing.T) {
	m := errorMapper{logger: slog.New(slog.DiscardHandler)}
	ctx := t.Context()

	tests := []struct {
		name       string
		err        error
		wantCode   connect.Code
		wantReason string
		wantField  string
		wantRetry  int64
		wantMsg    string
	}{
		{name: "reason", err: domain.Unauthenticated(domain.ReasonInvalidCredentials), wantCode: connect.CodeUnauthenticated, wantReason: domain.ReasonInvalidCredentials},
		{name: "fields", err: domain.Invalid(domain.FieldViolation{Field: "email", Reason: "invalid_format"}), wantCode: connect.CodeInvalidArgument, wantField: "email"},
		{name: "conflict", err: fmt.Errorf("create user: %w", domain.Conflict("username")), wantCode: connect.CodeAlreadyExists, wantField: "username"},
		{name: "rate limit", err: domain.RateLimited(1500 * time.Millisecond), wantCode: connect.CodeResourceExhausted, wantRetry: 1500},
		{name: "bare sentinel", err: fmt.Errorf("load: %w", domain.ErrNotFound), wantCode: connect.CodeNotFound},
		{name: "safe message", err: &domain.Error{Kind: domain.ErrFailedPrecondition, Reason: "email_delivery_failed", Message: "535 auth failed"}, wantCode: connect.CodeFailedPrecondition, wantReason: "email_delivery_failed", wantMsg: "535 auth failed"},
		{name: "unexpected", err: errors.New("disk on fire at /var/lib/secret"), wantCode: connect.CodeInternal, wantMsg: "internal error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ce *connect.Error
			if !errors.As(m.toConnect(ctx, tt.err), &ce) {
				t.Fatal("not a connect error")
			}
			if ce.Code() != tt.wantCode {
				t.Errorf("code = %v, want %v", ce.Code(), tt.wantCode)
			}
			if tt.wantMsg != "" && ce.Message() != tt.wantMsg {
				t.Errorf("message = %q, want %q", ce.Message(), tt.wantMsg)
			}
			var reason dislacksov1.ErrorReason
			var fields dislacksov1.FieldViolations
			var limited dislacksov1.RateLimited
			for _, d := range ce.Details() {
				switch d.Type {
				case "dislackso.v1.ErrorReason":
					_ = proto.Unmarshal(d.Value, &reason)
				case "dislackso.v1.FieldViolations":
					_ = proto.Unmarshal(d.Value, &fields)
				case "dislackso.v1.RateLimited":
					_ = proto.Unmarshal(d.Value, &limited)
				}
			}
			if reason.GetReason() != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason.GetReason(), tt.wantReason)
			}
			if tt.wantField != "" && (len(fields.GetViolations()) != 1 || fields.GetViolations()[0].GetField() != tt.wantField) {
				t.Errorf("fields = %v, want %s", fields.GetViolations(), tt.wantField)
			}
			if limited.GetRetryAfterMs() != tt.wantRetry {
				t.Errorf("retry = %d, want %d", limited.GetRetryAfterMs(), tt.wantRetry)
			}
		})
	}
}
