package app

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1/dislacksov1connect"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/config"
)

func TestServeAndShutdown(t *testing.T) {
	webDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<!doctype html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		DataDir: t.TempDir(),
		HTTP:    config.HTTP{Addr: "127.0.0.1:0"},
		Log:     config.Log{Level: "info", Format: config.LogFormatText},
		Web:     config.Web{Dir: webDir},
	}
	a, err := New(cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	addr, err := a.Listen(ctx)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- a.Serve(ctx) }()

	// A client of our own, so its idle connections can be closed before
	// shutdown; Server.Shutdown waits up to 5 s for never-used connections.
	httpClient := &http.Client{Transport: &http.Transport{}}
	base := "http://" + addr.String()
	for _, p := range []string{"/healthz", "/readyz", "/", "/some/client/route"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+p, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", p, resp.StatusCode)
		}
	}

	client := dislacksov1connect.NewHealthServiceClient(
		connect.NewClient(connecthttp.NewTransport(httpClient, base+"/rpc")),
	)
	res, err := client.Check(ctx, &dislacksov1.CheckRequest{})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.GetStatus() != dislacksov1.ServingStatus_SERVING_STATUS_SERVING {
		t.Errorf("status = %v, want SERVING", res.GetStatus())
	}

	httpClient.CloseIdleConnections()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after cancellation")
	}
}

func TestNewRejectsMissingWebDir(t *testing.T) {
	cfg := config.Config{HTTP: config.HTTP{Addr: "127.0.0.1:0"}, Web: config.Web{Dir: filepath.Join(t.TempDir(), "missing")}}
	if _, err := New(cfg, slog.New(slog.DiscardHandler)); err == nil {
		t.Fatal("expected an error for a missing web dir")
	}
}
