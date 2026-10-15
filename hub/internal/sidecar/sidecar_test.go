package sidecar

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// The test binary doubles as the supervised process: with HELPER_LISTEN set
// it listens there until killed; with HELPER_EXIT it exits at once.
func TestMain(m *testing.M) {
	if addr := os.Getenv("HELPER_LISTEN"); addr != "" {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			os.Exit(2)
		}
		for {
			c, err := ln.Accept()
			if err != nil {
				os.Exit(3)
			}
			_ = c.Close()
		}
	}
	if os.Getenv("HELPER_EXIT") != "" {
		os.Exit(7)
	}
	os.Exit(m.Run())
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func dialHealth(addr string) func(context.Context) error {
	return func(ctx context.Context) error {
		var d net.Dialer
		c, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return err
		}
		return c.Close()
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSupervisorRestartsAndStops(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	addr := freeAddr(t)
	var mu sync.Mutex
	var readies []int
	logPath := filepath.Join(t.TempDir(), "logs", "helper.log")
	s := New(Spec{
		Name: "helper", Path: exe, Env: []string{"HELPER_LISTEN=" + addr}, LogPath: logPath,
		Health: dialHealth(addr),
		OnReady: func(_ context.Context, restarts int) {
			mu.Lock()
			readies = append(readies, restarts)
			mu.Unlock()
		},
	}, slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	waitFor(t, "first start", s.Ready)
	first := s.Status().PID
	p, err := os.FindProcess(first)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Kill(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "restart", func() bool { st := s.Status(); return st.State == StateRunning && st.PID != first })
	if st := s.Status(); st.Restarts != 1 || st.LastError != "" {
		t.Errorf("status after restart = %+v", st)
	}
	mu.Lock()
	if len(readies) != 2 || readies[0] != 0 || readies[1] != 1 {
		t.Errorf("OnReady calls = %v", readies)
	}
	mu.Unlock()

	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); st.State != StateStopped || st.PID != 0 {
		t.Errorf("status after stop = %+v", st)
	}
	if err := dialHealth(addr)(t.Context()); err == nil {
		t.Error("the process still serves after the supervisor stopped")
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("log: %v", err)
	}
}

func TestSupervisorReportsCrashes(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	s := New(Spec{
		Name: "crasher", Path: exe, Env: []string{"HELPER_EXIT=1"},
		LogPath: filepath.Join(t.TempDir(), "crasher.log"),
		Health:  func(context.Context) error { return errors.New("never") },
	}, slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { _ = s.Run(ctx) }()
	waitFor(t, "a crash", func() bool { return s.Status().Restarts > 0 })
	if st := s.Status(); st.State != StateRestarting || st.LastError == "" {
		t.Errorf("status = %+v", st)
	}

	missing := New(Spec{Name: "missing", Path: filepath.Join(t.TempDir(), "nope")}, slog.New(slog.DiscardHandler))
	if err := missing.Run(t.Context()); err == nil {
		t.Error("ran a binary that does not exist")
	}
}
