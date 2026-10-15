package rpc

import (
	"context"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1/dislacksov1connect"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/guild"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/wire"
)

type GuildService interface {
	Create(ctx context.Context, actor domain.Actor, locale, name string, iconFileID domain.ID) (domain.GuildState, error)
	Update(ctx context.Context, actor domain.Actor, guildID domain.ID, p guild.Patch) (domain.Guild, error)
	Delete(ctx context.Context, actor domain.Actor, guildID domain.ID, confirmName string) error
	Leave(ctx context.Context, actor domain.Actor, guildID domain.ID) error
	TransferOwnership(ctx context.Context, actor domain.Actor, guildID, targetID domain.ID) (domain.Guild, error)
	SetMemberRole(ctx context.Context, actor domain.Actor, guildID, targetID domain.ID, role domain.GuildRole) (domain.Member, error)
	Members(ctx context.Context, actor domain.Actor, guildID domain.ID) (guild.MemberList, error)
}

type guildHandler struct {
	dislacksov1connect.UnimplementedGuildServiceHandler
	errorMapper
	svc GuildService
}

func (h *guildHandler) CreateGuild(ctx context.Context, req *dislacksov1.CreateGuildRequest) (*dislacksov1.CreateGuildResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	icon, err := optionalID(req.GetIconFileId(), "icon_file_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	state, err := h.svc.Create(ctx, actor, clientFrom(ctx).Locale, req.GetName(), icon)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.CreateGuildResponse{State: wire.GuildState(state)}, nil
}

func (h *guildHandler) UpdateGuild(ctx context.Context, req *dislacksov1.UpdateGuildRequest) (*dislacksov1.UpdateGuildResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	id, err := domain.ParseID(req.GetGuildId(), "guild_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	p := guild.Patch{Name: req.Name}
	if req.IconFileId != nil {
		icon, err := optionalID(req.GetIconFileId(), "icon_file_id")
		if err != nil {
			return nil, h.toConnect(ctx, err)
		}
		p.IconFileID = &icon
	}
	g, err := h.svc.Update(ctx, actor, id, p)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.UpdateGuildResponse{Guild: wire.Guild(g)}, nil
}

func (h *guildHandler) DeleteGuild(ctx context.Context, req *dislacksov1.DeleteGuildRequest) (*dislacksov1.DeleteGuildResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	id, err := domain.ParseID(req.GetGuildId(), "guild_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	if err := h.svc.Delete(ctx, actor, id, req.GetConfirmName()); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.DeleteGuildResponse{}, nil
}

func (h *guildHandler) LeaveGuild(ctx context.Context, req *dislacksov1.LeaveGuildRequest) (*dislacksov1.LeaveGuildResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	id, err := domain.ParseID(req.GetGuildId(), "guild_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	if err := h.svc.Leave(ctx, actor, id); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.LeaveGuildResponse{}, nil
}

func (h *guildHandler) TransferOwnership(ctx context.Context, req *dislacksov1.TransferOwnershipRequest) (*dislacksov1.TransferOwnershipResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	guildID, userID, err := guildAndUser(req.GetGuildId(), req.GetUserId())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	g, err := h.svc.TransferOwnership(ctx, actor, guildID, userID)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.TransferOwnershipResponse{Guild: wire.Guild(g)}, nil
}

func (h *guildHandler) SetMemberRole(ctx context.Context, req *dislacksov1.SetMemberRoleRequest) (*dislacksov1.SetMemberRoleResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	guildID, userID, err := guildAndUser(req.GetGuildId(), req.GetUserId())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	m, err := h.svc.SetMemberRole(ctx, actor, guildID, userID, wire.RoleFromProto(req.GetRole()))
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.SetMemberRoleResponse{Member: wire.Member(m)}, nil
}

func (h *guildHandler) ListMembers(ctx context.Context, req *dislacksov1.ListMembersRequest) (*dislacksov1.ListMembersResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	id, err := domain.ParseID(req.GetGuildId(), "guild_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	list, err := h.svc.Members(ctx, actor, id)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.ListMembersResponse{
		Members:   wire.Members(list.Members),
		Users:     wire.Users(list.Users),
		Presences: wire.Presences(list.Presences),
	}, nil
}

func guildAndUser(rawGuild, rawUser string) (domain.ID, domain.ID, error) {
	guildID, err := domain.ParseID(rawGuild, "guild_id")
	if err != nil {
		return 0, 0, err
	}
	userID, err := domain.ParseID(rawUser, "user_id")
	if err != nil {
		return 0, 0, err
	}
	return guildID, userID, nil
}
