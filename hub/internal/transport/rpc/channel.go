package rpc

import (
	"context"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1/dislacksov1connect"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/channel"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/wire"
)

type ChannelService interface {
	Create(ctx context.Context, actor domain.Actor, guildID domain.ID, t domain.ChannelType, name string, parentID domain.ID) (domain.Channel, error)
	Update(ctx context.Context, actor domain.Actor, channelID domain.ID, p channel.Patch) (domain.Channel, error)
	Delete(ctx context.Context, actor domain.Actor, channelID domain.ID) error
	Reorder(ctx context.Context, actor domain.Actor, guildID domain.ID, placements []channel.Placement) ([]domain.Channel, error)
}

type channelHandler struct {
	dislacksov1connect.UnimplementedChannelServiceHandler
	errorMapper
	svc ChannelService
}

func (h *channelHandler) CreateChannel(ctx context.Context, req *dislacksov1.CreateChannelRequest) (*dislacksov1.CreateChannelResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	guildID, err := domain.ParseID(req.GetGuildId(), "guild_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	parentID, err := optionalID(req.GetParentId(), "parent_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	c, err := h.svc.Create(ctx, actor, guildID, wire.ChannelTypeFromProto(req.GetType()), req.GetName(), parentID)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.CreateChannelResponse{Channel: wire.Channel(c)}, nil
}

func (h *channelHandler) UpdateChannel(ctx context.Context, req *dislacksov1.UpdateChannelRequest) (*dislacksov1.UpdateChannelResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	id, err := domain.ParseID(req.GetChannelId(), "channel_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	p := channel.Patch{
		Name:            req.Name,
		Topic:           req.Topic,
		DrawingAllowed:  req.DrawingAllowed,
		MaxShareQuality: req.MaxShareQuality,
	}
	if req.UserLimit != nil {
		limit := int(req.GetUserLimit())
		p.UserLimit = &limit
	}
	c, err := h.svc.Update(ctx, actor, id, p)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.UpdateChannelResponse{Channel: wire.Channel(c)}, nil
}

func (h *channelHandler) DeleteChannel(ctx context.Context, req *dislacksov1.DeleteChannelRequest) (*dislacksov1.DeleteChannelResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	id, err := domain.ParseID(req.GetChannelId(), "channel_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	if err := h.svc.Delete(ctx, actor, id); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.DeleteChannelResponse{}, nil
}

func (h *channelHandler) ReorderChannels(ctx context.Context, req *dislacksov1.ReorderChannelsRequest) (*dislacksov1.ReorderChannelsResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	guildID, err := domain.ParseID(req.GetGuildId(), "guild_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	placements := make([]channel.Placement, len(req.GetPlacements()))
	for i, p := range req.GetPlacements() {
		id, err := domain.ParseID(p.GetChannelId(), "placements")
		if err != nil {
			return nil, h.toConnect(ctx, err)
		}
		parent, err := optionalID(p.GetParentId(), "placements")
		if err != nil {
			return nil, h.toConnect(ctx, err)
		}
		placements[i] = channel.Placement{ChannelID: id, ParentID: parent}
	}
	channels, err := h.svc.Reorder(ctx, actor, guildID, placements)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.ReorderChannelsResponse{Channels: wire.Channels(channels)}, nil
}
