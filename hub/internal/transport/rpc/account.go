package rpc

import (
	"context"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1/dislacksov1connect"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

type AccountService interface {
	Me(ctx context.Context, actor domain.Actor) (domain.User, error)
	UpdateProfile(ctx context.Context, actor domain.Actor, displayName string) (domain.User, error)
	SetStatus(ctx context.Context, actor domain.Actor, pref domain.StatusPref, custom string) (domain.User, error)
	ChangePassword(ctx context.Context, actor domain.Actor, current, next string) error
	Sessions(ctx context.Context, actor domain.Actor) ([]domain.Session, error)
	RevokeSession(ctx context.Context, actor domain.Actor, id domain.ID) error
	RevokeOtherSessions(ctx context.Context, actor domain.Actor) (int, error)
}

type accountHandler struct {
	dislacksov1connect.UnimplementedAccountServiceHandler
	errorMapper
	svc AccountService
}

func (h *accountHandler) GetMe(ctx context.Context, _ *dislacksov1.GetMeRequest) (*dislacksov1.GetMeResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	u, err := h.svc.Me(ctx, actor)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.GetMeResponse{Me: toMe(u)}, nil
}

func (h *accountHandler) UpdateProfile(ctx context.Context, req *dislacksov1.UpdateProfileRequest) (*dislacksov1.UpdateProfileResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	u, err := h.svc.UpdateProfile(ctx, actor, req.GetDisplayName())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.UpdateProfileResponse{Me: toMe(u)}, nil
}

func (h *accountHandler) ChangePassword(ctx context.Context, req *dislacksov1.ChangePasswordRequest) (*dislacksov1.ChangePasswordResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.ChangePassword(ctx, actor, req.GetCurrentPassword(), req.GetNewPassword()); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.ChangePasswordResponse{}, nil
}

func (h *accountHandler) SetStatusPreference(ctx context.Context, req *dislacksov1.SetStatusPreferenceRequest) (*dislacksov1.SetStatusPreferenceResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	u, err := h.svc.SetStatus(ctx, actor, statusFromProto(req.GetStatus()), req.GetCustomStatus())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.SetStatusPreferenceResponse{Me: toMe(u)}, nil
}

func (h *accountHandler) ListSessions(ctx context.Context, _ *dislacksov1.ListSessionsRequest) (*dislacksov1.ListSessionsResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	sessions, err := h.svc.Sessions(ctx, actor)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	out := make([]*dislacksov1.Session, len(sessions))
	for i, s := range sessions {
		out[i] = sessionToProto(s, actor.SessionID)
	}
	return &dislacksov1.ListSessionsResponse{Sessions: out}, nil
}

func (h *accountHandler) RevokeSession(ctx context.Context, req *dislacksov1.RevokeSessionRequest) (*dislacksov1.RevokeSessionResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	id, err := domain.ParseID(req.GetSessionId(), "session_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	if err := h.svc.RevokeSession(ctx, actor, id); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	if id == actor.SessionID {
		endSession(ctx)
	}
	return &dislacksov1.RevokeSessionResponse{}, nil
}

func (h *accountHandler) RevokeOtherSessions(ctx context.Context, _ *dislacksov1.RevokeOtherSessionsRequest) (*dislacksov1.RevokeOtherSessionsResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	n, err := h.svc.RevokeOtherSessions(ctx, actor)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.RevokeOtherSessionsResponse{Revoked: clampInt32(n)}, nil
}
