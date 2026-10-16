package rpc

import (
	"context"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1/dislacksov1connect"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/voice"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/wire"
)

type VoiceService interface {
	Join(ctx context.Context, actor domain.Actor, channelID domain.ID, selfMute, selfDeaf bool) (voice.Join, error)
	Leave(ctx context.Context, actor domain.Actor, sessionID string) error
	MoveMember(ctx context.Context, actor domain.Actor, guildID, userID, channelID domain.ID) error
	SetServerMute(ctx context.Context, actor domain.Actor, guildID, userID domain.ID, mute bool) (domain.Member, error)
	SetServerDeafen(ctx context.Context, actor domain.Actor, guildID, userID domain.ID, deaf bool) (domain.Member, error)
	DisconnectMember(ctx context.Context, actor domain.Actor, guildID, userID domain.ID) error
}

type voiceHandler struct {
	dislacksov1connect.UnimplementedVoiceServiceHandler
	errorMapper
	svc VoiceService
}

func (h *voiceHandler) Join(ctx context.Context, req *dislacksov1.JoinRequest) (*dislacksov1.JoinResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	channelID, err := domain.ParseID(req.GetChannelId(), "channel_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	j, err := h.svc.Join(ctx, actor, channelID, req.GetSelfMute(), req.GetSelfDeaf())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.JoinResponse{
		Url:        j.URL,
		Token:      j.Token,
		IceServers: iceServersToProto(j.ICE),
		State:      wire.VoiceState(j.State),
		Limits:     &dislacksov1.VoiceLimits{MicBitrateMaxKbps: wire.ClampInt32(j.MicBitrateMaxKbps)},
	}, nil
}

func (h *voiceHandler) Leave(ctx context.Context, req *dislacksov1.LeaveRequest) (*dislacksov1.LeaveResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.Leave(ctx, actor, req.GetVoiceSessionId()); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.LeaveResponse{}, nil
}

func (h *voiceHandler) MoveMember(ctx context.Context, req *dislacksov1.MoveMemberRequest) (*dislacksov1.MoveMemberResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	guildID, userID, err := guildAndUser(req.GetGuildId(), req.GetUserId())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	channelID, err := domain.ParseID(req.GetChannelId(), "channel_id")
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	if err := h.svc.MoveMember(ctx, actor, guildID, userID, channelID); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.MoveMemberResponse{}, nil
}

func (h *voiceHandler) SetServerMute(ctx context.Context, req *dislacksov1.SetServerMuteRequest) (*dislacksov1.SetServerMuteResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	guildID, userID, err := guildAndUser(req.GetGuildId(), req.GetUserId())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	m, err := h.svc.SetServerMute(ctx, actor, guildID, userID, req.GetMute())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.SetServerMuteResponse{Member: wire.Member(m)}, nil
}

func (h *voiceHandler) SetServerDeafen(ctx context.Context, req *dislacksov1.SetServerDeafenRequest) (*dislacksov1.SetServerDeafenResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	guildID, userID, err := guildAndUser(req.GetGuildId(), req.GetUserId())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	m, err := h.svc.SetServerDeafen(ctx, actor, guildID, userID, req.GetDeaf())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.SetServerDeafenResponse{Member: wire.Member(m)}, nil
}

func (h *voiceHandler) DisconnectMember(ctx context.Context, req *dislacksov1.DisconnectMemberRequest) (*dislacksov1.DisconnectMemberResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	guildID, userID, err := guildAndUser(req.GetGuildId(), req.GetUserId())
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	if err := h.svc.DisconnectMember(ctx, actor, guildID, userID); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.DisconnectMemberResponse{}, nil
}
