package rpc

import (
	"context"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1/dislacksov1connect"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/auth"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/setup"
)

type SetupService interface {
	CheckToken(ctx context.Context, client domain.ClientInfo, token string) error
	SendTestEmail(ctx context.Context, client domain.ClientInfo, token string, smtp domain.SMTPSettings, password, to string) error
	Complete(ctx context.Context, client domain.ClientInfo, in setup.CompleteInput) (auth.SignedIn, error)
}

type setupHandler struct {
	dislacksov1connect.UnimplementedSetupServiceHandler
	errorMapper
	svc SetupService
}

func (h *setupHandler) CheckSetupToken(ctx context.Context, req *dislacksov1.CheckSetupTokenRequest) (*dislacksov1.CheckSetupTokenResponse, error) {
	if err := h.svc.CheckToken(ctx, clientFrom(ctx), req.GetToken()); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.CheckSetupTokenResponse{}, nil
}

func (h *setupHandler) SendSetupTestEmail(ctx context.Context, req *dislacksov1.SendSetupTestEmailRequest) (*dislacksov1.SendSetupTestEmailResponse, error) {
	smtp, password := smtpFromProto(req.GetSmtp())
	if err := h.svc.SendTestEmail(ctx, clientFrom(ctx), req.GetToken(), smtp, password, req.GetTo()); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.SendSetupTestEmailResponse{}, nil
}

func (h *setupHandler) CompleteSetup(ctx context.Context, req *dislacksov1.CompleteSetupRequest) (*dislacksov1.CompleteSetupResponse, error) {
	in := setup.CompleteInput{
		Token:         req.GetToken(),
		InstanceName:  req.GetInstanceName(),
		PublicURL:     req.GetPublicUrl(),
		AdminUsername: req.GetAdminUsername(),
		AdminEmail:    req.GetAdminEmail(),
		AdminPassword: req.GetAdminPassword(),
	}
	if req.GetSmtp().GetEnabled() {
		smtp, password := smtpFromProto(req.GetSmtp())
		in.SMTP, in.SMTPPassword = &smtp, password
	}
	signed, err := h.svc.Complete(ctx, clientFrom(ctx), in)
	if err != nil {
		return nil, h.toConnect(ctx, renameField(err, "name", "instance_name"))
	}
	startSession(ctx, signed)
	return &dislacksov1.CompleteSetupResponse{Me: toMe(signed.User)}, nil
}

// renameField maps a service field name to this request's field name.
func renameField(err error, from, to string) error {
	if de, ok := domain.AsError(err); ok {
		for i, f := range de.Fields {
			if f.Field == from {
				de.Fields[i].Field = to
			}
		}
	}
	return err
}
