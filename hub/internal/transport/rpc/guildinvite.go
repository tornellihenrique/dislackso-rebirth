package rpc

import (
	"context"
	"time"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1/dislacksov1connect"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/invite"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/wire"
)

type GuildInviteService interface {
	CreateGuildInvite(ctx context.Context, actor domain.Actor, guildID domain.ID, maxAge time.Duration, maxUses int) (domain.InviteView, error)
	ListGuildInvites(ctx context.Context, actor domain.Actor, guildID domain.ID) ([]domain.InviteView, error)
	RevokeGuildInvite(ctx context.Context, actor domain.Actor, code string) error
	PreviewGuildInvite(ctx context.Context, client domain.ClientInfo, actor *domain.Actor, code string) (invite.Preview, error)
	AcceptGuildInvite(ctx context.Context, actor domain.Actor, code string) (domain.GuildState, error)
}

type inviteHandler struct {
	dislacksov1connect.UnimplementedInviteServiceHandler
	errorMapper
	svc GuildInviteService
}

func (h *inviteHandler) CreateInvite(ctx context.Context, req *dislacksov1.CreateInviteRequest) (*dislacksov1.CreateInviteResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	guildID, err := domain.ParseID(req.GetGuildId(), "guild_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	v, err := h.svc.CreateGuildInvite(ctx, actor, guildID, time.Duration(req.GetMaxAgeMs())*time.Millisecond, int(req.GetMaxUses()))
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.CreateInviteResponse{Invite: wire.Invite(v)}, nil
}

func (h *inviteHandler) ListInvites(ctx context.Context, req *dislacksov1.ListInvitesRequest) (*dislacksov1.ListInvitesResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	guildID, err := domain.ParseID(req.GetGuildId(), "guild_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	views, err := h.svc.ListGuildInvites(ctx, actor, guildID)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	out := make([]*dislacksov1.Invite, len(views))
	for i, v := range views {
		out[i] = wire.Invite(v)
	}
	return &dislacksov1.ListInvitesResponse{Invites: out}, nil
}

func (h *inviteHandler) RevokeInvite(ctx context.Context, req *dislacksov1.RevokeInviteRequest) (*dislacksov1.RevokeInviteResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.RevokeGuildInvite(ctx, actor, req.GetCode()); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.RevokeInviteResponse{}, nil
}

func (h *inviteHandler) PreviewInvite(ctx context.Context, req *dislacksov1.PreviewInviteRequest) (*dislacksov1.PreviewInviteResponse, error) {
	var actor *domain.Actor
	if p := requestFrom(ctx).principal; p != nil {
		actor = &p.Actor
	}
	p, err := h.svc.PreviewGuildInvite(ctx, clientFrom(ctx), actor, req.GetCode())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	out := &dislacksov1.InvitePreview{
		Code:        p.Invite.Code,
		GuildId:     p.Guild.ID.String(),
		GuildName:   p.Guild.Name,
		MemberCount: wire.ClampInt32(p.MemberCount),
		OnlineCount: wire.ClampInt32(p.OnlineCount),
		Member:      p.Member,
	}
	if p.Guild.IconFileID != 0 {
		// The landing page is shown signed out, so the icon comes through
		// the invite rather than the session-protected file URL.
		out.GuildIconUrl = "/api/invites/" + p.Invite.Code + "/icon"
	}
	if p.Invite.ExpiresAt != nil {
		out.ExpiresAt = p.Invite.ExpiresAt.UnixMilli()
	}
	if p.Inviter != nil {
		out.Inviter = wire.PublicUser(*p.Inviter)
	}
	return &dislacksov1.PreviewInviteResponse{Preview: out}, nil
}

func (h *inviteHandler) AcceptInvite(ctx context.Context, req *dislacksov1.AcceptInviteRequest) (*dislacksov1.AcceptInviteResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	state, err := h.svc.AcceptGuildInvite(ctx, actor, req.GetCode())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.AcceptInviteResponse{State: wire.GuildState(state)}, nil
}
