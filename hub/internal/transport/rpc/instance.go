package rpc

import (
	"context"
	"strings"
	"time"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1/dislacksov1connect"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/instance"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/invite"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/settings"
)

type InstanceService interface {
	Settings(ctx context.Context, actor domain.Actor) (instance.SettingsView, error)
	UpdateSettings(ctx context.Context, actor domain.Actor, p settings.Patch) (instance.SettingsView, error)
	SendTestEmail(ctx context.Context, actor domain.Actor, client domain.ClientInfo, to string) error
	Users(ctx context.Context, actor domain.Actor) ([]domain.User, error)
	SetUserDisabled(ctx context.Context, actor domain.Actor, userID domain.ID, disabled bool) (domain.User, error)
	SetUserInstanceAdmin(ctx context.Context, actor domain.Actor, userID domain.ID, admin bool) (domain.User, error)
	CreatePasswordResetLink(ctx context.Context, actor domain.Actor, userID domain.ID) (string, time.Time, error)
}

type InviteService interface {
	CreateInstanceInvite(ctx context.Context, actor domain.Actor, ttl time.Duration) (invite.CreatedInvite, error)
	ListInstanceInvites(ctx context.Context, actor domain.Actor) ([]invite.InstanceInvite, error)
	RevokeInstanceInvite(ctx context.Context, actor domain.Actor, id domain.ID) error
}

type instanceHandler struct {
	dislacksov1connect.UnimplementedInstanceServiceHandler
	errorMapper
	svc     InstanceService
	invites InviteService
}

func (h *instanceHandler) GetSettings(ctx context.Context, _ *dislacksov1.GetSettingsRequest) (*dislacksov1.GetSettingsResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := h.svc.Settings(ctx, actor)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.GetSettingsResponse{Settings: settingsToProto(v)}, nil
}

func (h *instanceHandler) UpdateSettings(ctx context.Context, req *dislacksov1.UpdateSettingsRequest) (*dislacksov1.UpdateSettingsResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	var p settings.Patch
	if req.Name != nil {
		name := strings.TrimSpace(req.GetName())
		p.Name = &name
	}
	if req.PublicUrl != nil {
		u := req.GetPublicUrl()
		p.PublicURL = &u
	}
	if req.GetRegistrationMode() != dislacksov1.RegistrationMode_REGISTRATION_MODE_UNSPECIFIED {
		m := registrationModeFromProto(req.GetRegistrationMode())
		p.RegistrationMode = &m
	}
	if req.MembersCanInvite != nil {
		v := req.GetMembersCanInvite()
		p.MembersCanInvite = &v
	}
	if req.GetSmtp() != nil {
		s, password := smtpFromProto(req.GetSmtp())
		p.SMTP, p.SMTPPassword = &s, password
	}
	v, err := h.svc.UpdateSettings(ctx, actor, p)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.UpdateSettingsResponse{Settings: settingsToProto(v)}, nil
}

func (h *instanceHandler) SendTestEmail(ctx context.Context, req *dislacksov1.SendTestEmailRequest) (*dislacksov1.SendTestEmailResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.SendTestEmail(ctx, actor, clientFrom(ctx), req.GetTo()); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.SendTestEmailResponse{}, nil
}

func (h *instanceHandler) ListUsers(ctx context.Context, _ *dislacksov1.ListUsersRequest) (*dislacksov1.ListUsersResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	users, err := h.svc.Users(ctx, actor)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	out := make([]*dislacksov1.InstanceUser, len(users))
	for i, u := range users {
		out[i] = instanceUserToProto(u)
	}
	return &dislacksov1.ListUsersResponse{Users: out}, nil
}

func (h *instanceHandler) SetUserDisabled(ctx context.Context, req *dislacksov1.SetUserDisabledRequest) (*dislacksov1.SetUserDisabledResponse, error) {
	actor, id, err := h.actorAndUser(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	u, err := h.svc.SetUserDisabled(ctx, actor, id, req.GetDisabled())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.SetUserDisabledResponse{User: instanceUserToProto(u)}, nil
}

func (h *instanceHandler) SetUserInstanceAdmin(ctx context.Context, req *dislacksov1.SetUserInstanceAdminRequest) (*dislacksov1.SetUserInstanceAdminResponse, error) {
	actor, id, err := h.actorAndUser(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	u, err := h.svc.SetUserInstanceAdmin(ctx, actor, id, req.GetInstanceAdmin())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.SetUserInstanceAdminResponse{User: instanceUserToProto(u)}, nil
}

func (h *instanceHandler) CreatePasswordResetLink(ctx context.Context, req *dislacksov1.CreatePasswordResetLinkRequest) (*dislacksov1.CreatePasswordResetLinkResponse, error) {
	actor, id, err := h.actorAndUser(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	link, expires, err := h.svc.CreatePasswordResetLink(ctx, actor, id)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.CreatePasswordResetLinkResponse{Url: link, ExpiresAt: expires.UnixMilli()}, nil
}

func (h *instanceHandler) CreateInstanceInvite(ctx context.Context, req *dislacksov1.CreateInstanceInviteRequest) (*dislacksov1.CreateInstanceInviteResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	created, err := h.invites.CreateInstanceInvite(ctx, actor, time.Duration(req.GetExpiresInMs())*time.Millisecond)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.CreateInstanceInviteResponse{
		Invite: inviteToProto(created.Invite),
		Code:   created.Code,
		Url:    created.URL,
	}, nil
}

func (h *instanceHandler) ListInstanceInvites(ctx context.Context, _ *dislacksov1.ListInstanceInvitesRequest) (*dislacksov1.ListInstanceInvitesResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	invites, err := h.invites.ListInstanceInvites(ctx, actor)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	out := make([]*dislacksov1.InstanceInvite, len(invites))
	for i, inv := range invites {
		out[i] = inviteToProto(inv)
	}
	return &dislacksov1.ListInstanceInvitesResponse{Invites: out}, nil
}

func (h *instanceHandler) RevokeInstanceInvite(ctx context.Context, req *dislacksov1.RevokeInstanceInviteRequest) (*dislacksov1.RevokeInstanceInviteResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	id, err := domain.ParseID(req.GetInviteId(), "invite_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	if err := h.invites.RevokeInstanceInvite(ctx, actor, id); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.RevokeInstanceInviteResponse{}, nil
}

func (h *instanceHandler) actorAndUser(ctx context.Context, rawID string) (domain.Actor, domain.ID, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return domain.Actor{}, 0, err
	}
	id, err := domain.ParseID(rawID, "user_id")
	if err != nil {
		return domain.Actor{}, 0, h.toConnect(ctx, err)
	}
	return actor, id, nil
}
