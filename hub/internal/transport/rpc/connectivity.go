package rpc

import (
	"context"
	"strings"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1/dislacksov1connect"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/connectivity"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/exposure"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/relay"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/wire"
)

type ConnectivityService interface {
	Get(ctx context.Context, actor domain.Actor) (connectivity.View, error)
	SetExposure(ctx context.Context, actor domain.Actor, p exposure.Patch) (connectivity.View, error)
	SetTURN(ctx context.Context, actor domain.Actor, p relay.Patch) (connectivity.View, error)
	RunSelfTest(ctx context.Context, actor domain.Actor) (domain.SelfTest, error)
	RelayTest(ctx context.Context, actor domain.Actor) (connectivity.RelayTest, error)
	SetHost(ctx context.Context, actor domain.Actor, client domain.ClientInfo, keepAwake, runAtLogin *bool) (domain.HostOptions, error)
	AddFirewallRule(ctx context.Context, actor domain.Actor, client domain.ClientInfo) error
}

type connectivityHandler struct {
	dislacksov1connect.UnimplementedConnectivityServiceHandler
	errorMapper
	svc ConnectivityService
}

func (h *connectivityHandler) GetConnectivity(ctx context.Context, _ *dislacksov1.GetConnectivityRequest) (*dislacksov1.GetConnectivityResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := h.svc.Get(ctx, actor)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.GetConnectivityResponse{Connectivity: connectivityToProto(v, clientFrom(ctx))}, nil
}

func (h *connectivityHandler) SetExposure(ctx context.Context, req *dislacksov1.SetExposureRequest) (*dislacksov1.SetExposureResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := h.svc.SetExposure(ctx, actor, exposureFromProto(req.GetExposure()))
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.SetExposureResponse{Connectivity: connectivityToProto(v, clientFrom(ctx))}, nil
}

func (h *connectivityHandler) SetTurn(ctx context.Context, req *dislacksov1.SetTurnRequest) (*dislacksov1.SetTurnResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	t := req.GetTurn()
	urls := make([]string, 0, len(t.GetUrls()))
	for _, u := range t.GetUrls() {
		if u = strings.TrimSpace(u); u != "" {
			urls = append(urls, u)
		}
	}
	v, err := h.svc.SetTURN(ctx, actor, relay.Patch{
		Settings: domain.TURNSettings{
			Kind:            wire.TURNKindFromProto(t.GetKind()),
			CloudflareKeyID: strings.TrimSpace(t.GetCloudflareKeyId()),
			URLs:            urls,
			Username:        strings.TrimSpace(t.GetUsername()),
			BudgetOverride:  t.GetBudgetOverride(),
		},
		Secret:   strings.TrimSpace(t.GetSecret()),
		BudgetGB: int(t.GetMonthlyBudgetGb()),
	})
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.SetTurnResponse{Connectivity: connectivityToProto(v, clientFrom(ctx))}, nil
}

func (h *connectivityHandler) RunSelfTest(ctx context.Context, _ *dislacksov1.RunSelfTestRequest) (*dislacksov1.RunSelfTestResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	t, err := h.svc.RunSelfTest(ctx, actor)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.RunSelfTestResponse{Report: wire.SelfTest(t)}, nil
}

func (h *connectivityHandler) StartRelayTest(ctx context.Context, _ *dislacksov1.StartRelayTestRequest) (*dislacksov1.StartRelayTestResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	t, err := h.svc.RelayTest(ctx, actor)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.StartRelayTestResponse{Url: t.URL, Token: t.Token, IceServers: iceServersToProto(t.ICE)}, nil
}

func (h *connectivityHandler) SetHostOptions(ctx context.Context, req *dislacksov1.SetHostOptionsRequest) (*dislacksov1.SetHostOptionsResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	client := clientFrom(ctx)
	o, err := h.svc.SetHost(ctx, actor, client, req.KeepAwake, req.RunAtLogin)
	if err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.SetHostOptionsResponse{Host: hostOptionsToProto(o, client)}, nil
}

func (h *connectivityHandler) AddFirewallRule(ctx context.Context, _ *dislacksov1.AddFirewallRuleRequest) (*dislacksov1.AddFirewallRuleResponse, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.AddFirewallRule(ctx, actor, clientFrom(ctx)); err != nil {
		return nil, h.toConnect(ctx, err)
	}
	return &dislacksov1.AddFirewallRuleResponse{}, nil
}

func iceServersToProto(servers []ports.IceServer) []*dislacksov1.IceServer {
	out := make([]*dislacksov1.IceServer, len(servers))
	for i, s := range servers {
		out[i] = &dislacksov1.IceServer{Urls: s.URLs, Username: s.Username, Credential: s.Credential}
	}
	return out
}

func exposureFromProto(p *dislacksov1.ExposureSettings) exposure.Patch {
	return exposure.Patch{
		Settings: domain.ExposureSettings{
			Kind:              wire.ExposureKindFromProto(p.GetKind()),
			TailscaleHostname: strings.TrimSpace(p.GetTailscaleHostname()),
			TunnelURL:         strings.TrimSpace(p.GetTunnelUrl()),
			DirectDomain:      strings.ToLower(strings.TrimSpace(p.GetDirectDomain())),
			DirectEmail:       strings.TrimSpace(p.GetDirectEmail()),
			DirectAddr:        strings.TrimSpace(p.GetDirectAddr()),
			DynamicDNS:        wire.DynamicDNSKindFromProto(p.GetDynamicDns()),
			DynamicDNSName:    strings.ToLower(strings.TrimSpace(p.GetDynamicDnsName())),
			LANAddr:           strings.TrimSpace(p.GetLanAddr()),
		},
		TunnelToken: strings.TrimSpace(p.GetTunnelToken()),
		DNSToken:    strings.TrimSpace(p.GetDnsToken()),
	}
}

func hostOptionsToProto(o domain.HostOptions, client domain.ClientInfo) *dislacksov1.HostOptions {
	return &dislacksov1.HostOptions{
		KeepAwake:           o.KeepAwake,
		RunAtLogin:          o.RunAtLogin,
		RunAtLoginSupported: o.RunAtLoginSupported,
		FirewallSupported:   o.FirewallSupported,
		Local:               client.IsLocal(),
	}
}

func connectivityToProto(v connectivity.View, client domain.ClientInfo) *dislacksov1.Connectivity {
	c := &dislacksov1.Connectivity{
		Exposure: &dislacksov1.ExposureSettings{
			Kind:              wire.ExposureKind(v.Exposure.Kind),
			TailscaleHostname: v.Exposure.TailscaleHostname,
			TunnelUrl:         v.Exposure.TunnelURL,
			DirectDomain:      v.Exposure.DirectDomain,
			DirectEmail:       v.Exposure.DirectEmail,
			DirectAddr:        v.Exposure.DirectAddr,
			DynamicDns:        wire.DynamicDNSKind(v.Exposure.DynamicDNS),
			DynamicDnsName:    v.Exposure.DynamicDNSName,
			LanAddr:           v.Exposure.LANAddr,
			TunnelTokenSet:    v.TunnelTokenSet,
			DnsTokenSet:       v.DNSTokenSet,
		},
		Status: wire.ExposureStatus(v.Status),
		Turn: &dislacksov1.TurnSettings{
			Kind:            wire.TURNKind(v.TURN.Kind),
			CloudflareKeyId: v.TURN.CloudflareKeyID,
			Urls:            v.TURN.URLs,
			Username:        v.TURN.Username,
			SecretSet:       v.TURNSecretSet,
			BudgetOverride:  v.TURN.BudgetOverride,
			MonthlyBudgetGb: wire.ClampInt32(v.BudgetGB),
		},
		Usage: &dislacksov1.TurnUsage{
			MonthBytes:      v.Usage.MonthBytes,
			ProjectedBytes:  v.Usage.ProjectedBytes,
			BudgetGb:        wire.ClampInt32(v.Usage.BudgetGB),
			OverBudget:      v.Usage.OverBudget,
			VoiceSessions:   wire.ClampInt32(v.Usage.VoiceSessions),
			RelayedSessions: wire.ClampInt32(v.Usage.RelayedSessions),
		},
		Host: hostOptionsToProto(v.Host, client),
	}
	if v.SelfTest != nil {
		c.SelfTest = wire.SelfTest(*v.SelfTest)
	}
	return c
}
