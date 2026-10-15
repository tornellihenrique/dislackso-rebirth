package rpc

import (
	"context"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1/dislacksov1connect"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/moderation"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/wire"
)

type ModerationService interface {
	Kick(ctx context.Context, actor domain.Actor, guildID, targetID domain.ID, reason string) error
	Ban(ctx context.Context, actor domain.Actor, guildID, targetID domain.ID, reason string) error
	Unban(ctx context.Context, actor domain.Actor, guildID, targetID domain.ID) error
	Bans(ctx context.Context, actor domain.Actor, guildID domain.ID) ([]domain.BanView, error)
	AuditLog(ctx context.Context, actor domain.Actor, guildID, before domain.ID, limit int) (moderation.AuditPage, error)
}

type moderationHandler struct {
	dislacksov1connect.UnimplementedModerationServiceHandler
	errorMapper
	svc ModerationService
}

func (h *moderationHandler) KickMember(ctx context.Context, req *dislacksov1.KickMemberRequest) (*dislacksov1.KickMemberResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	guildID, userID, err := guildAndUser(req.GetGuildId(), req.GetUserId())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	if err := h.svc.Kick(ctx, actor, guildID, userID, req.GetReason()); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.KickMemberResponse{}, nil
}

func (h *moderationHandler) BanMember(ctx context.Context, req *dislacksov1.BanMemberRequest) (*dislacksov1.BanMemberResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	guildID, userID, err := guildAndUser(req.GetGuildId(), req.GetUserId())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	if err := h.svc.Ban(ctx, actor, guildID, userID, req.GetReason()); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.BanMemberResponse{}, nil
}

func (h *moderationHandler) UnbanMember(ctx context.Context, req *dislacksov1.UnbanMemberRequest) (*dislacksov1.UnbanMemberResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	guildID, userID, err := guildAndUser(req.GetGuildId(), req.GetUserId())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	if err := h.svc.Unban(ctx, actor, guildID, userID); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.UnbanMemberResponse{}, nil
}

func (h *moderationHandler) ListBans(ctx context.Context, req *dislacksov1.ListBansRequest) (*dislacksov1.ListBansResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	guildID, err := domain.ParseID(req.GetGuildId(), "guild_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	bans, err := h.svc.Bans(ctx, actor, guildID)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	out := make([]*dislacksov1.Ban, len(bans))
	for i, b := range bans {
		out[i] = wire.Ban(b)
	}
	return &dislacksov1.ListBansResponse{Bans: out}, nil
}

func (h *moderationHandler) ListAuditLog(ctx context.Context, req *dislacksov1.ListAuditLogRequest) (*dislacksov1.ListAuditLogResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	guildID, err := domain.ParseID(req.GetGuildId(), "guild_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	before, err := optionalID(req.GetBefore(), "before")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	page, err := h.svc.AuditLog(ctx, actor, guildID, before, int(req.GetLimit()))
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	entries := make([]*dislacksov1.AuditEntry, len(page.Entries))
	for i, e := range page.Entries {
		entries[i] = wire.AuditEntry(e)
	}
	return &dislacksov1.ListAuditLogResponse{Entries: entries, Users: wire.Users(page.Users)}, nil
}
