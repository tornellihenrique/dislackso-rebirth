package ingress

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func TestClientIP(t *testing.T) {
	tunnel := Info{Name: "tunnel", Secure: true, ClientIP: func(r *http.Request) netip.Addr {
		ip, _ := netip.ParseAddr(r.Header.Get("X-Client"))
		return ip
	}}
	tests := []struct {
		name      string
		info      Info
		remote    string
		header    string
		wantIP    string
		wantLocal bool
		secure    bool
	}{
		{name: "local from this machine", info: Local, remote: "127.0.0.1:5000", wantIP: "127.0.0.1", wantLocal: true},
		{name: "local from the network", info: Local, remote: "192.168.1.9:5000", wantIP: "192.168.1.9"},
		{name: "tunnel names the client", info: tunnel, remote: "127.0.0.1:5000", header: "198.51.100.7", wantIP: "198.51.100.7", secure: true},
		// cloudflared connects from this machine; that must never pass for a local visitor.
		{name: "tunnel hop", info: tunnel, remote: "127.0.0.1:5000", wantIP: "invalid IP", secure: true},
		{name: "tunnel claiming loopback", info: tunnel, remote: "127.0.0.1:5000", header: "127.0.0.1", wantIP: "invalid IP", secure: true},
		{name: "public listener", info: Info{Name: "lan"}, remote: "[::ffff:192.168.1.9]:5000", wantIP: "192.168.1.9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(context.WithValue(t.Context(), infoKey{}, tt.info), http.MethodGet, "/", http.NoBody)
			r.RemoteAddr = tt.remote
			if tt.header != "" {
				r.Header.Set("X-Client", tt.header)
			}
			if got := ClientIP(r).String(); got != tt.wantIP {
				t.Errorf("ClientIP = %s, want %s", got, tt.wantIP)
			}
			if got := IsLocal(r); got != tt.wantLocal {
				t.Errorf("IsLocal = %v, want %v", got, tt.wantLocal)
			}
			if got := Secure(r); got != tt.secure {
				t.Errorf("Secure = %v, want %v", got, tt.secure)
			}
		})
	}
}

func TestRequestsWithoutAnIngressCountAsLocal(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r.RemoteAddr = "127.0.0.1:5000"
	if !IsLocal(r) || From(r).Name != "local" {
		t.Errorf("From = %+v", From(r))
	}
}

func TestServeTagsRequestsAndStops(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	info := Info{Name: "lan"}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, From(r).Name)
	})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, ln, handler, info, slog.New(slog.DiscardHandler), time.Second) }()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+ln.Addr().String(), http.NoBody)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "lan" {
		t.Errorf("handler saw ingress %q, want lan", body)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve = %v, want nil after cancel", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not stop")
	}
}
