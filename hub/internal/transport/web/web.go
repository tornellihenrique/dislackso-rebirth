// Package web serves the plain HTTP routes: the embedded web client, health
// endpoints, and (later) uploads, file downloads, the LiveKit signaling proxy
// and webhooks.
package web

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/health"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/clientip"
)

type HealthChecker interface {
	Check(ctx context.Context) health.Report
}

type SetupStatus interface {
	Pending(ctx context.Context) (bool, error)
}

type Options struct {
	// Files is the built web client, rooted at index.html.
	Files    fs.FS
	Health   HealthChecker
	Setup    SetupStatus
	ClientIP clientip.Resolver
}

// Register adds the routes to mux. The SPA catch-all is registered last on
// "/", so more specific routes registered elsewhere take precedence.
func Register(mux *http.ServeMux, o Options) {
	mux.HandleFunc("GET /healthz", liveness)
	mux.Handle("GET /readyz", readiness(o.Health))
	spa := newSPAHandler(o.Files)
	mux.Handle("GET /setup", setupPage(spa, o.Setup, o.ClientIP))
	mux.Handle("/", spa)
}

// setupPage serves the wizard only to the host machine and only until setup
// completes; the setup RPCs enforce the same rules.
func setupPage(spa http.Handler, setup SetupStatus, resolveIP clientip.Resolver) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !resolveIP(r).IsLoopback() {
			http.NotFound(w, r)
			return
		}
		pending, err := setup.Pending(r.Context())
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if !pending {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		spa.ServeHTTP(w, r)
	})
}

// liveness answers as long as the process serves HTTP.
func liveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type readinessBody struct {
	Status string `json:"status"`
	// Failing lists probe names only: probe errors can reveal host details.
	Failing []string `json:"failing,omitempty"`
}

// readiness reports 503 while any probe fails.
func readiness(checker HealthChecker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		report := checker.Check(r.Context())
		if report.Status == health.StatusServing {
			writeJSON(w, http.StatusOK, readinessBody{Status: "ready"})
			return
		}
		failing := make([]string, 0, len(report.Failures))
		for _, f := range report.Failures {
			failing = append(failing, f.Probe)
		}
		writeJSON(w, http.StatusServiceUnavailable, readinessBody{Status: "degraded", Failing: failing})
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
