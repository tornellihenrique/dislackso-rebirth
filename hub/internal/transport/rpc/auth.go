package rpc

import (
	"context"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1/dislacksov1connect"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/auth"
)

type AuthService interface {
	SetupState(ctx context.Context, client domain.ClientInfo) (auth.SetupState, error)
	Register(ctx context.Context, client domain.ClientInfo, in auth.RegisterInput) (auth.SignedIn, error)
	Login(ctx context.Context, client domain.ClientInfo, login, password string) (auth.SignedIn, error)
	Logout(ctx context.Context, actor domain.Actor) error
	VerifyEmail(ctx context.Context, client domain.ClientInfo, token string) error
	ResendVerification(ctx context.Context, client domain.ClientInfo, actor domain.Actor) error
	RequestPasswordReset(ctx context.Context, client domain.ClientInfo, email string) error
	ResetPassword(ctx context.Context, client domain.ClientInfo, token, newPassword string) error
}

type authHandler struct {
	dislacksov1connect.UnimplementedAuthServiceHandler
	errorMapper
	svc AuthService
}

func (h *authHandler) GetSetupState(ctx context.Context, _ *dislacksov1.GetSetupStateRequest) (*dislacksov1.GetSetupStateResponse, error) {
	st, err := h.svc.SetupState(ctx, clientFrom(ctx))
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.GetSetupStateResponse{
		SetupComplete:      st.Complete,
		InstanceName:       st.InstanceName,
		RegistrationMode:   registrationModeToProto(st.RegistrationMode),
		EmailVerification:  st.EmailVerification,
		SetupAvailableHere: st.SetupAvailableHere,
	}, nil
}

func (h *authHandler) Register(ctx context.Context, req *dislacksov1.RegisterRequest) (*dislacksov1.RegisterResponse, error) {
	signed, err := h.svc.Register(ctx, clientFrom(ctx), auth.RegisterInput{
		Username:   req.GetUsername(),
		Email:      req.GetEmail(),
		Password:   req.GetPassword(),
		InviteCode: req.GetInviteCode(),
	})
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	startSession(ctx, signed)
	return &dislacksov1.RegisterResponse{Me: toMe(signed.User)}, nil
}

func (h *authHandler) Login(ctx context.Context, req *dislacksov1.LoginRequest) (*dislacksov1.LoginResponse, error) {
	signed, err := h.svc.Login(ctx, clientFrom(ctx), req.GetLogin(), req.GetPassword())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	startSession(ctx, signed)
	return &dislacksov1.LoginResponse{Me: toMe(signed.User)}, nil
}

// Logout always clears the cookie, even when the session was already gone.
func (h *authHandler) Logout(ctx context.Context, _ *dislacksov1.LogoutRequest) (*dislacksov1.LogoutResponse, error) {
	endSession(ctx)
	if p := requestFrom(ctx).principal; p != nil {
		if err := h.svc.Logout(ctx, p.Actor); err != nil {
			return nil, h.toConnect(ctx, err)
		}
	}
	return &dislacksov1.LogoutResponse{}, nil
}

func (h *authHandler) VerifyEmail(ctx context.Context, req *dislacksov1.VerifyEmailRequest) (*dislacksov1.VerifyEmailResponse, error) {
	if err := h.svc.VerifyEmail(ctx, clientFrom(ctx), req.GetToken()); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.VerifyEmailResponse{}, nil
}

func (h *authHandler) ResendVerification(ctx context.Context, _ *dislacksov1.ResendVerificationRequest) (*dislacksov1.ResendVerificationResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.ResendVerification(ctx, clientFrom(ctx), actor); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.ResendVerificationResponse{}, nil
}

func (h *authHandler) RequestPasswordReset(ctx context.Context, req *dislacksov1.RequestPasswordResetRequest) (*dislacksov1.RequestPasswordResetResponse, error) {
	if err := h.svc.RequestPasswordReset(ctx, clientFrom(ctx), req.GetEmail()); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.RequestPasswordResetResponse{}, nil
}

func (h *authHandler) ResetPassword(ctx context.Context, req *dislacksov1.ResetPasswordRequest) (*dislacksov1.ResetPasswordResponse, error) {
	if err := h.svc.ResetPassword(ctx, clientFrom(ctx), req.GetToken(), req.GetNewPassword()); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	// Every session of the account was revoked, possibly including this one.
	endSession(ctx)
	return &dislacksov1.ResetPasswordResponse{}, nil
}
