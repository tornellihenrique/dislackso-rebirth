package rpc

import (
	"context"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1/dislacksov1connect"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/health"
)

type HealthChecker interface {
	Check(ctx context.Context) health.Report
}

type HealthHandler struct {
	dislacksov1connect.UnimplementedHealthServiceHandler

	checker HealthChecker
}

func NewHealthHandler(checker HealthChecker) *HealthHandler {
	return &HealthHandler{checker: checker}
}

func (h *HealthHandler) Check(ctx context.Context, _ *dislacksov1.CheckRequest) (*dislacksov1.CheckResponse, error) {
	r := h.checker.Check(ctx)
	return &dislacksov1.CheckResponse{
		Status:     servingStatus(r.Status),
		Version:    r.Version,
		StartedAt:  r.StartedAt.UnixMilli(),
		ServerTime: r.Now.UnixMilli(),
	}, nil
}

func servingStatus(s health.Status) dislacksov1.ServingStatus {
	switch s {
	case health.StatusServing:
		return dislacksov1.ServingStatus_SERVING_STATUS_SERVING
	case health.StatusDegraded:
		return dislacksov1.ServingStatus_SERVING_STATUS_DEGRADED
	default:
		return dislacksov1.ServingStatus_SERVING_STATUS_UNSPECIFIED
	}
}
