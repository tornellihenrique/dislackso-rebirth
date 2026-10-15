package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"google.golang.org/protobuf/proto"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1/dislacksov1connect"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/config"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/rpc"
)

const setupToken = "test-setup-token"

type running struct {
	base string
	app  *App
}

// start runs a hub on a free port; opts adjust its config (voice is off
// unless an option turns it on).
func start(t *testing.T, opts ...func(*config.Config)) *running {
	t.Helper()
	webDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<!doctype html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		DataDir: filepath.Join(t.TempDir(), "data"),
		HTTP:    config.HTTP{Addr: "127.0.0.1:0"},
		Log:     config.Log{Level: "info", Format: config.LogFormatText},
		Web:     config.Web{Dir: webDir},
		Setup:   config.Setup{Token: setupToken},
		LiveKit: config.LiveKit{Disabled: true, Port: 7880},
	}
	for _, o := range opts {
		o(&cfg)
	}
	ctx, cancel := context.WithCancel(t.Context())
	a, err := New(ctx, cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		cancel()
		t.Fatalf("New: %v", err)
	}
	addr, err := a.Listen(ctx)
	if err != nil {
		cancel()
		t.Fatalf("Listen: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- a.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Serve: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("Serve did not return after cancellation")
		}
		if err := a.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return &running{base: "http://" + addr.String(), app: a}
}

// browser is an HTTP client with its own cookie jar, like one browser profile.
type browser struct {
	base       string
	http       *http.Client
	auth       dislacksov1connect.AuthServiceClient
	account    dislacksov1connect.AccountServiceClient
	instance   dislacksov1connect.InstanceServiceClient
	setup      dislacksov1connect.SetupServiceClient
	guilds     dislacksov1connect.GuildServiceClient
	channels   dislacksov1connect.ChannelServiceClient
	invites    dislacksov1connect.InviteServiceClient
	moderation dislacksov1connect.ModerationServiceClient
	voice      dislacksov1connect.VoiceServiceClient
}

func (r *running) browser(t *testing.T) *browser {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	// Closing idle connections before shutdown keeps Server.Shutdown from
	// waiting on never-used ones.
	hc := &http.Client{Jar: jar, Transport: &http.Transport{}, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	t.Cleanup(hc.CloseIdleConnections)
	c := connect.NewClient(connecthttp.NewTransport(hc, r.base+rpc.PathPrefix))
	return &browser{
		base:       r.base,
		http:       hc,
		auth:       dislacksov1connect.NewAuthServiceClient(c),
		account:    dislacksov1connect.NewAccountServiceClient(c),
		instance:   dislacksov1connect.NewInstanceServiceClient(c),
		setup:      dislacksov1connect.NewSetupServiceClient(c),
		guilds:     dislacksov1connect.NewGuildServiceClient(c),
		channels:   dislacksov1connect.NewChannelServiceClient(c),
		invites:    dislacksov1connect.NewInviteServiceClient(c),
		moderation: dislacksov1connect.NewModerationServiceClient(c),
		voice:      dislacksov1connect.NewVoiceServiceClient(c),
	}
}

// get returns the status and headers of a GET; the body is discarded.
func (b *browser) get(t *testing.T, url string) (int, http.Header) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := b.http.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode, resp.Header
}

func requireCode(t *testing.T, err error, code connect.Code) {
	t.Helper()
	if ce, ok := errors.AsType[*connect.Error](err); !ok || ce.Code() != code {
		t.Fatalf("err = %v, want %v", err, code)
	}
}

func completeSetup(t *testing.T, b *browser) *dislacksov1.Me {
	t.Helper()
	res, err := b.setup.CompleteSetup(t.Context(), &dislacksov1.CompleteSetupRequest{
		Token:         setupToken,
		InstanceName:  "Team",
		PublicUrl:     "http://localhost:7300",
		AdminUsername: "admin",
		AdminEmail:    "admin@example.com",
		AdminPassword: "correct horse battery",
	})
	if err != nil {
		t.Fatalf("CompleteSetup: %v", err)
	}
	return res.GetMe()
}

func TestServesHealthAndClientRoutes(t *testing.T) {
	r := start(t)
	b := r.browser(t)
	for _, p := range []string{"/healthz", "/readyz", "/", "/some/client/route"} {
		if status, _ := b.get(t, r.base+p); status != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", p, status)
		}
	}
	health := dislacksov1connect.NewHealthServiceClient(connect.NewClient(connecthttp.NewTransport(b.http, r.base+rpc.PathPrefix)))
	res, err := health.Check(t.Context(), &dislacksov1.CheckRequest{})
	if err != nil || res.GetStatus() != dislacksov1.ServingStatus_SERVING_STATUS_SERVING {
		t.Errorf("Check = %v, %v", res, err)
	}
}

func TestSetupFlow(t *testing.T) {
	r := start(t)
	admin := r.browser(t)
	ctx := t.Context()

	state, err := admin.auth.GetSetupState(ctx, &dislacksov1.GetSetupStateRequest{})
	if err != nil || state.GetSetupComplete() || !state.GetSetupAvailableHere() {
		t.Fatalf("before setup: %v, %v", state, err)
	}
	if status, _ := admin.get(t, r.base+"/setup"); status != http.StatusOK {
		t.Errorf("GET /setup before setup = %d", status)
	}
	_, err = admin.auth.Register(ctx, &dislacksov1.RegisterRequest{Username: "early", Email: "e@example.com", Password: "correct horse"})
	requireCode(t, err, connect.CodeFailedPrecondition)
	_, err = admin.setup.CheckSetupToken(ctx, &dislacksov1.CheckSetupTokenRequest{Token: "wrong"})
	requireCode(t, err, connect.CodePermissionDenied)

	me := completeSetup(t, admin)
	if !me.GetInstanceAdmin() || me.GetUser().GetUsername() != "admin" {
		t.Errorf("admin = %v", me)
	}
	got, err := admin.account.GetMe(ctx, &dislacksov1.GetMeRequest{})
	if err != nil || got.GetMe().GetUser().GetId() != me.GetUser().GetId() {
		t.Fatalf("the setup response must sign the admin in: %v, %v", got, err)
	}

	status, header := admin.get(t, r.base+"/setup")
	if status != http.StatusSeeOther || header.Get("Location") != "/" {
		t.Errorf("GET /setup after setup = %d → %q, want a redirect home", status, header.Get("Location"))
	}
	_, err = admin.setup.CheckSetupToken(ctx, &dislacksov1.CheckSetupTokenRequest{Token: setupToken})
	requireCode(t, err, connect.CodeFailedPrecondition)
	_, err = admin.setup.CompleteSetup(ctx, &dislacksov1.CompleteSetupRequest{Token: setupToken})
	requireCode(t, err, connect.CodeFailedPrecondition)
}

func TestRegistrationLoginAndLogout(t *testing.T) {
	r := start(t)
	ctx := t.Context()
	admin := r.browser(t)
	completeSetup(t, admin)

	newcomer := r.browser(t)
	_, err := newcomer.auth.Register(ctx, &dislacksov1.RegisterRequest{Username: "ana", Email: "ana@example.com", Password: "correct horse"})
	requireCode(t, err, connect.CodeInvalidArgument)
	if !hasViolation(t, err, "invite_code", "required") {
		t.Errorf("err = %v, want invite_code required", err)
	}

	invite, err := admin.instance.CreateInstanceInvite(ctx, &dislacksov1.CreateInstanceInviteRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(invite.GetUrl(), "http://localhost:7300/register?invite=") {
		t.Errorf("invite url = %s", invite.GetUrl())
	}
	_, err = newcomer.instance.CreateInstanceInvite(ctx, &dislacksov1.CreateInstanceInviteRequest{})
	requireCode(t, err, connect.CodeUnauthenticated)

	reg, err := newcomer.auth.Register(ctx, &dislacksov1.RegisterRequest{
		Username: "ana", Email: "ana@example.com", Password: "correct horse", InviteCode: invite.GetCode(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if reg.GetMe().GetEmailVerified() || reg.GetMe().GetInstanceAdmin() {
		t.Errorf("new account = %v", reg.GetMe())
	}
	_, err = newcomer.instance.ListUsers(ctx, &dislacksov1.ListUsersRequest{})
	requireCode(t, err, connect.CodePermissionDenied)

	stranger := r.browser(t)
	_, unknown := stranger.auth.Login(ctx, &dislacksov1.LoginRequest{Login: "nobody", Password: "correct horse"})
	_, wrong := stranger.auth.Login(ctx, &dislacksov1.LoginRequest{Login: "ana", Password: "wrong password"})
	requireCode(t, unknown, connect.CodeUnauthenticated)
	requireCode(t, wrong, connect.CodeUnauthenticated)
	if unknown.Error() != wrong.Error() {
		t.Errorf("unknown user (%v) and wrong password (%v) differ", unknown, wrong)
	}

	if _, err := stranger.auth.Login(ctx, &dislacksov1.LoginRequest{Login: "ANA@example.com", Password: "correct horse"}); err != nil {
		t.Fatal(err)
	}
	sessions, err := stranger.account.ListSessions(ctx, &dislacksov1.ListSessionsRequest{})
	if err != nil || len(sessions.GetSessions()) != 2 {
		t.Fatalf("sessions = %v, %v; want the registration and the login", sessions, err)
	}

	if _, err := stranger.auth.Logout(ctx, &dislacksov1.LogoutRequest{}); err != nil {
		t.Fatal(err)
	}
	_, err = stranger.account.GetMe(ctx, &dislacksov1.GetMeRequest{})
	requireCode(t, err, connect.CodeUnauthenticated)
	if _, err := newcomer.account.GetMe(ctx, &dislacksov1.GetMeRequest{}); err != nil {
		t.Errorf("logging out one browser signed out another: %v", err)
	}
}

func TestCrossOriginRequestsAreRefused(t *testing.T) {
	r := start(t)
	b := r.browser(t)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		r.base+rpc.PathPrefix+dislacksov1connect.AuthServiceLoginProcedure, strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("Origin", "https://evil.example")
	resp, err := b.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestNewRejectsMissingWebDir(t *testing.T) {
	cfg := config.Config{DataDir: t.TempDir(), HTTP: config.HTTP{Addr: "127.0.0.1:0"}, Web: config.Web{Dir: filepath.Join(t.TempDir(), "missing")}}
	if _, err := New(t.Context(), cfg, slog.New(slog.DiscardHandler)); err == nil {
		t.Fatal("expected an error for a missing web dir")
	}
}

func hasViolation(t *testing.T, err error, field, reason string) bool {
	t.Helper()
	ce, ok := errors.AsType[*connect.Error](err)
	if !ok {
		return false
	}
	for _, d := range ce.Details() {
		if d.Type != "dislackso.v1.FieldViolations" {
			continue
		}
		var v dislacksov1.FieldViolations
		if err := proto.Unmarshal(d.Value, &v); err != nil {
			t.Fatal(err)
		}
		for _, f := range v.GetViolations() {
			if f.GetField() == field && f.GetReason() == reason {
				return true
			}
		}
	}
	return false
}

func TestLoginIsRateLimited(t *testing.T) {
	r := start(t)
	ctx := t.Context()
	admin := r.browser(t)
	completeSetup(t, admin)

	attacker := r.browser(t)
	var last error
	for range 6 {
		_, last = attacker.auth.Login(ctx, &dislacksov1.LoginRequest{Login: "admin", Password: "guess"})
	}
	requireCode(t, last, connect.CodeResourceExhausted)
	ce, _ := errors.AsType[*connect.Error](last)
	found := false
	for _, d := range ce.Details() {
		if d.Type == "dislackso.v1.RateLimited" {
			var rl dislacksov1.RateLimited
			if err := proto.Unmarshal(d.Value, &rl); err != nil {
				t.Fatal(err)
			}
			found = rl.GetRetryAfterMs() > 0
		}
	}
	if !found {
		t.Errorf("details = %v, want a positive retry delay", ce.Details())
	}
}
