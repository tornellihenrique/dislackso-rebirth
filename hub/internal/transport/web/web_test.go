package web

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/health"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/ingress"
)

type stubChecker struct{ report health.Report }

func (s stubChecker) Check(context.Context) health.Report { return s.report }

var builtClient = fstest.MapFS{
	"index.html":         {Data: []byte("<!doctype html><html><head><title>app</title></head></html>")},
	"favicon.svg":        {Data: []byte("<svg/>")},
	"assets/app-abc.js":  {Data: []byte("console.log(1)")},
	"assets/app-abc.css": {Data: []byte("body{}")},
	".gitkeep":           {Data: nil},
}

type stubSetup struct{ pending bool }

func (s stubSetup) Pending(context.Context) (bool, error) { return s.pending, nil }

func newMux(files fstest.MapFS, report health.Report) http.Handler {
	return newMuxWithSetup(files, report, false)
}

func newMuxWithSetup(files fstest.MapFS, report health.Report, pending bool) http.Handler {
	mux := http.NewServeMux()
	Register(mux, Options{Files: files, Health: stubChecker{report: report}, Setup: stubSetup{pending: pending}, ClientIP: ingress.ClientIP})
	return SecurityHeaders(mux)
}

func TestSetupPage(t *testing.T) {
	tests := []struct {
		name     string
		remote   string
		pending  bool
		wantCode int
	}{
		{"local while pending", "127.0.0.1:5000", true, http.StatusOK},
		{"ipv6 loopback while pending", "[::1]:5000", true, http.StatusOK},
		{"remote while pending", "203.0.113.9:5000", true, http.StatusNotFound},
		{"local after setup", "127.0.0.1:5000", false, http.StatusSeeOther},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/setup?token=x", http.NoBody)
			req.RemoteAddr = tt.remote
			rec := httptest.NewRecorder()
			newMuxWithSetup(builtClient, health.Report{}, tt.pending).ServeHTTP(rec, req)
			if rec.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantCode)
			}
		})
	}
}

func TestSPA(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		path      string
		files     fstest.MapFS
		wantCode  int
		wantBody  string
		wantCache string
		wantCSP   bool
	}{
		{name: "root serves index", path: "/", wantCode: http.StatusOK, wantBody: "<title>app</title>", wantCache: "no-store", wantCSP: true},
		{name: "index.html is not redirected", path: "/index.html", wantCode: http.StatusOK, wantBody: "<title>app</title>", wantCSP: true},
		{name: "client route falls back to index", path: "/channels/123/456", wantCode: http.StatusOK, wantBody: "<title>app</title>", wantCSP: true},
		{name: "fingerprinted asset is immutable", path: "/assets/app-abc.js", wantCode: http.StatusOK, wantBody: "console.log(1)", wantCache: "public, max-age=31536000, immutable"},
		{name: "root file is revalidated", path: "/favicon.svg", wantCode: http.StatusOK, wantBody: "<svg/>", wantCache: "no-cache"},
		{name: "missing asset is 404, not index", path: "/assets/missing.js", wantCode: http.StatusNotFound},
		{name: "dotfiles are hidden", path: "/.gitkeep", wantCode: http.StatusNotFound},
		{name: "directory is not listed", path: "/assets/", wantCode: http.StatusOK, wantBody: "<title>app</title>", wantCSP: true},
		{name: "unknown rpc path is 404", path: "/rpc/dislackso.v1.Nope/Nope", wantCode: http.StatusNotFound},
		{name: "unknown api path is 404", path: "/api/nope", wantCode: http.StatusNotFound},
		{name: "gateway path is reserved", path: "/gateway", wantCode: http.StatusNotFound},
		{name: "POST is not allowed", method: http.MethodPost, path: "/", wantCode: http.StatusMethodNotAllowed},
		{name: "missing build explains itself", path: "/", files: fstest.MapFS{}, wantCode: http.StatusServiceUnavailable, wantBody: "web client not built"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := tt.files
			if files == nil {
				files = builtClient
			}
			method := tt.method
			if method == "" {
				method = http.MethodGet
			}
			rec := httptest.NewRecorder()
			newMux(files, health.Report{Status: health.StatusServing}).ServeHTTP(rec, httptest.NewRequest(method, tt.path, http.NoBody))

			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantCode)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want it to contain %q", rec.Body.String(), tt.wantBody)
			}
			if tt.wantCache != "" && rec.Header().Get("Cache-Control") != tt.wantCache {
				t.Errorf("Cache-Control = %q, want %q", rec.Header().Get("Cache-Control"), tt.wantCache)
			}
			if got := rec.Header().Get("Content-Security-Policy") != ""; got != tt.wantCSP {
				t.Errorf("CSP present = %v, want %v", got, tt.wantCSP)
			}
			if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Error("missing X-Content-Type-Options: nosniff")
			}
		})
	}
}

// The style nonce in the CSP header is the one handed to the page, and a
// new one comes with every response.
func TestSPAStyleNonce(t *testing.T) {
	nonceOf := func() (header, meta string) {
		rec := httptest.NewRecorder()
		newMux(builtClient, health.Report{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
		_, after, _ := strings.Cut(rec.Header().Get("Content-Security-Policy"), "'nonce-")
		header, _, _ = strings.Cut(after, "'")
		_, after, _ = strings.Cut(rec.Body.String(), `<meta name="csp-nonce" content="`)
		meta, _, _ = strings.Cut(after, `"`)
		return header, meta
	}
	h1, m1 := nonceOf()
	h2, _ := nonceOf()
	if len(h1) < 20 || h1 != m1 {
		t.Errorf("header nonce %q, page nonce %q", h1, m1)
	}
	if h1 == h2 {
		t.Error("the nonce was reused")
	}
}

func TestSPAGzippedAssets(t *testing.T) {
	files := fstest.MapFS{
		"index.html":             {Data: []byte("<title>app</title>")},
		"assets/big-abc.wasm":    {Data: []byte("plain")},
		"assets/big-abc.wasm.gz": {Data: []byte("gzipped")},
		"assets/small-abc.js":    {Data: []byte("small")},
	}
	tests := []struct {
		name         string
		path         string
		accept       string
		wantBody     string
		wantEncoding string
		wantVary     bool
	}{
		{"gzip copy for a client taking gzip", "/assets/big-abc.wasm", "gzip, deflate, br", "gzipped", "gzip", true},
		{"plain for a client refusing gzip", "/assets/big-abc.wasm", "gzip;q=0, br", "plain", "", true},
		{"plain without Accept-Encoding", "/assets/big-abc.wasm", "", "plain", "", true},
		{"plain when the build has no gzip copy", "/assets/small-abc.js", "gzip", "small", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, http.NoBody)
			if tt.accept != "" {
				req.Header.Set("Accept-Encoding", tt.accept)
			}
			rec := httptest.NewRecorder()
			newSPAHandler(files).ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || rec.Body.String() != tt.wantBody {
				t.Fatalf("got %d %q, want 200 %q", rec.Code, rec.Body.String(), tt.wantBody)
			}
			if got := rec.Header().Get("Content-Encoding"); got != tt.wantEncoding {
				t.Errorf("Content-Encoding = %q, want %q", got, tt.wantEncoding)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/wasm" && strings.HasSuffix(tt.path, ".wasm") {
				t.Errorf("Content-Type = %q, want application/wasm", got)
			}
			if got := rec.Header().Get("Vary") == "Accept-Encoding"; got != tt.wantVary {
				t.Errorf("Vary: Accept-Encoding = %v, want %v", got, tt.wantVary)
			}
		})
	}
}

func TestSPATraversalStaysInsideTheBuild(t *testing.T) {
	for _, p := range []string{"/../../etc/passwd", "/assets/../../secret.txt", "/..%2f..%2fsecret.txt"} {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.URL.Path = p
		rec := httptest.NewRecorder()
		newSPAHandler(builtClient).ServeHTTP(rec, req)
		if rec.Code == http.StatusOK && !strings.Contains(rec.Body.String(), "<title>app</title>") {
			t.Errorf("%s served %q", p, rec.Body.String())
		}
	}
}

func TestHealthEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		report   health.Report
		wantCode int
		wantBody string
	}{
		{name: "liveness", path: "/healthz", wantCode: http.StatusOK, wantBody: `{"status":"ok"}`},
		{name: "liveness ignores probes", path: "/healthz", report: health.Report{Status: health.StatusDegraded}, wantCode: http.StatusOK, wantBody: `{"status":"ok"}`},
		{name: "ready", path: "/readyz", report: health.Report{Status: health.StatusServing}, wantCode: http.StatusOK, wantBody: `{"status":"ready"}`},
		{
			name: "degraded hides probe errors",
			path: "/readyz",
			report: health.Report{
				Status:   health.StatusDegraded,
				Failures: []health.ProbeFailure{{Probe: "db", Err: errors.New(`open C:\secret\data.db: locked`)}},
			},
			wantCode: http.StatusServiceUnavailable,
			wantBody: `{"status":"degraded","failing":["db"]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newMux(builtClient, tt.report).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, http.NoBody))
			if rec.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantCode)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != tt.wantBody {
				t.Errorf("body = %s, want %s", got, tt.wantBody)
			}
		})
	}
}

func TestAccessLogKeepsResponseControllerFeatures(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	h := AccessLog(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "chunk")
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("Flush through middleware: %v", err)
		}
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if !rec.Flushed {
		t.Error("response was not flushed")
	}
}
