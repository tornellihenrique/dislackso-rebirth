package rpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1/dislacksov1connect"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/health"
)

type stubChecker struct{ report health.Report }

func (s stubChecker) Check(context.Context) health.Report { return s.report }

func newTestServer(t *testing.T, report health.Report) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	Mount(mux, Services{Health: NewHealthHandler(stubChecker{report: report})})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestHealthCheck(t *testing.T) {
	tests := []struct {
		name   string
		report health.Report
		want   *dislacksov1.CheckResponse
	}{
		{
			name:   "serving",
			report: health.Report{Status: health.StatusServing, Version: "v1.2.3", StartedAt: time.UnixMilli(1_000), Now: time.UnixMilli(2_000)},
			want: &dislacksov1.CheckResponse{
				Status:     dislacksov1.ServingStatus_SERVING_STATUS_SERVING,
				Version:    "v1.2.3",
				StartedAt:  1_000,
				ServerTime: 2_000,
			},
		},
		{
			name:   "degraded",
			report: health.Report{Status: health.StatusDegraded, Version: "dev", StartedAt: time.UnixMilli(1), Now: time.UnixMilli(2)},
			want: &dislacksov1.CheckResponse{
				Status:     dislacksov1.ServingStatus_SERVING_STATUS_DEGRADED,
				Version:    "dev",
				StartedAt:  1,
				ServerTime: 2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t, tt.report)
			client := dislacksov1connect.NewHealthServiceClient(
				connect.NewClient(connecthttp.NewTransport(srv.Client(), srv.URL+PathPrefix)),
			)

			got, err := client.Check(t.Context(), &dislacksov1.CheckRequest{})
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			if d := cmp.Diff(tt.want, got, protocmp.Transform()); d != "" {
				t.Errorf("response mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestRequiresConnectProtocolHeader(t *testing.T) {
	srv := newTestServer(t, health.Report{Status: health.StatusServing})
	url := srv.URL + PathPrefix + dislacksov1connect.HealthServiceCheckProcedure

	tests := []struct {
		name       string
		header     map[string]string
		wantStatus int
	}{
		{name: "without protocol header", header: map[string]string{"Content-Type": "application/json"}, wantStatus: http.StatusBadRequest},
		{name: "with protocol header", header: map[string]string{"Content-Type": "application/json", "Connect-Protocol-Version": "1"}, wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, strings.NewReader("{}"))
			if err != nil {
				t.Fatal(err)
			}
			for k, v := range tt.header {
				req.Header.Set(k, v)
			}
			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestUnknownServiceIsNotFound(t *testing.T) {
	srv := newTestServer(t, health.Report{})
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+PathPrefix+"/dislackso.v1.NopeService/Nope", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
