package publicprobe

import (
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

// doh answers DNS-over-HTTPS JSON queries from a table of host → A record.
func doh(t *testing.T, records map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, ok := records[r.URL.Query().Get("name")]
		w.Header().Set("Content-Type", "application/dns-json")
		switch {
		case !ok:
			_, _ = w.Write([]byte(`{"Status":3}`))
		case r.URL.Query().Get("type") == "A":
			_, _ = w.Write([]byte(`{"Status":0,"Answer":[{"type":1,"data":"` + ip + `"}]}`))
		default:
			_, _ = w.Write([]byte(`{"Status":0}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestReachable(t *testing.T) {
	var status atomic.Int32
	status.Store(http.StatusOK)
	site := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(int(status.Load()))
	}))
	defer site.Close()
	u, _ := url.Parse(site.URL)
	port := u.Port()
	roots := x509.NewCertPool()
	roots.AddCert(site.Certificate())

	dns := doh(t, map[string]string{"example.com": "127.0.0.1"})
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer broken.Close()

	p := Prober{Resolvers: []string{broken.URL, dns.URL}, RootCAs: roots}
	ctx := t.Context()
	// The certificate is for example.com, so the request must go to the
	// resolved address while checking the name.
	if err := p.Reachable(ctx, "https://example.com:"+port+"/"); err != nil {
		t.Fatalf("reachable site: %v", err)
	}

	status.Store(http.StatusServiceUnavailable)
	if err := p.Reachable(ctx, "https://example.com:"+port+"/"); err == nil {
		t.Error("a 503 counted as reachable")
	}

	err := p.Reachable(ctx, "https://not-yet.example.net:"+port+"/")
	if !errors.Is(err, ErrNotPublished) {
		t.Errorf("unpublished name err = %v, want ErrNotPublished", err)
	}

	untrusted := Prober{Resolvers: []string{dns.URL}}
	if err := untrusted.Reachable(ctx, "https://example.com:"+port+"/"); err == nil {
		t.Error("a certificate the system does not trust counted as reachable")
	}

	if err := p.Reachable(ctx, "http://example.com/"); err == nil {
		t.Error("plain http counted as reachable")
	}
}
