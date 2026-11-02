//go:build integration

package app

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/config"
)

// A public address that moves restarts LiveKit advertising the new one, and
// tells the hub the media server came back.
func TestMediaServerFollowsThePublicAddress(t *testing.T) {
	dir := t.TempDir()
	var public atomic.Value
	public.Store("203.0.113.1")
	var readies atomic.Int32
	m := &mediaServer{
		cfg:        config.LiveKit{Port: freePort(t), UDPPort: freePort(t), TCPPort: freePort(t)},
		findNodeIP: func(context.Context) string { return public.Load().(string) },
		publicIP: func(context.Context) (netip.Addr, error) {
			return netip.ParseAddr(public.Load().(string))
		},
		checkEvery: 20 * time.Millisecond,
		dataDir:    dir,
		logLevel:   "info",
		logger:     slog.New(slog.DiscardHandler),
		binary:     liveKitBinary(t),
		key:        "APIfollowsaddress",
		secret:     strings.Repeat("s", 40),
	}
	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	defer wg.Wait()
	defer cancel()
	hub := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: freePort(t)}
	if err := m.start(ctx, &wg, hub, func(context.Context, int) { readies.Add(1) }); err != nil {
		t.Fatal(err)
	}
	eventually(t, 30*time.Second, "the media server", m.Ready)
	advertises := func(ip string) bool {
		data, err := os.ReadFile(filepath.Join(dir, "livekit.yaml"))
		return err == nil && strings.Contains(string(data), "node_ip: "+ip)
	}
	if !advertises("203.0.113.1") {
		t.Fatal("livekit.yaml does not advertise the first address")
	}
	first := m.sup.Load().Status().PID

	public.Store("203.0.113.2")
	eventually(t, 30*time.Second, "a restart on the new address", func() bool {
		st := m.sup.Load().Status()
		return advertises("203.0.113.2") && st.PID != first && m.Ready()
	})
	if st := m.sup.Load().Status(); st.Restarts != 1 {
		t.Errorf("restarts = %d, want 1", st.Restarts)
	}
	eventually(t, 5*time.Second, "the hub told of it", func() bool { return readies.Load() == 2 })
}
