package wire

import (
	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

var exposureKinds = map[domain.ExposureKind]dislacksov1.ExposureKind{
	domain.ExposureLocal:            dislacksov1.ExposureKind_EXPOSURE_KIND_LOCAL,
	domain.ExposureTailscaleFunnel:  dislacksov1.ExposureKind_EXPOSURE_KIND_TAILSCALE_FUNNEL,
	domain.ExposureCloudflareQuick:  dislacksov1.ExposureKind_EXPOSURE_KIND_CLOUDFLARE_QUICK,
	domain.ExposureCloudflareTunnel: dislacksov1.ExposureKind_EXPOSURE_KIND_CLOUDFLARE_TUNNEL,
	domain.ExposureDirect:           dislacksov1.ExposureKind_EXPOSURE_KIND_DIRECT,
	domain.ExposureLAN:              dislacksov1.ExposureKind_EXPOSURE_KIND_LAN,
}

func ExposureKind(k domain.ExposureKind) dislacksov1.ExposureKind { return exposureKinds[k] }

func ExposureKindFromProto(k dislacksov1.ExposureKind) domain.ExposureKind {
	return reverse(exposureKinds, k)
}

var dynamicDNSKinds = map[domain.DynamicDNSKind]dislacksov1.DynamicDnsKind{
	domain.DynamicDNSNone:    dislacksov1.DynamicDnsKind_DYNAMIC_DNS_KIND_NONE,
	domain.DynamicDNSDuckDNS: dislacksov1.DynamicDnsKind_DYNAMIC_DNS_KIND_DUCKDNS,
}

func DynamicDNSKind(k domain.DynamicDNSKind) dislacksov1.DynamicDnsKind { return dynamicDNSKinds[k] }

func DynamicDNSKindFromProto(k dislacksov1.DynamicDnsKind) domain.DynamicDNSKind {
	return reverse(dynamicDNSKinds, k)
}

var exposureStates = map[domain.ExposureState]dislacksov1.ExposureState{
	domain.ExposureOff:        dislacksov1.ExposureState_EXPOSURE_STATE_OFF,
	domain.ExposureStarting:   dislacksov1.ExposureState_EXPOSURE_STATE_STARTING,
	domain.ExposureNeedsLogin: dislacksov1.ExposureState_EXPOSURE_STATE_NEEDS_LOGIN,
	domain.ExposureRunning:    dislacksov1.ExposureState_EXPOSURE_STATE_RUNNING,
	domain.ExposureFailed:     dislacksov1.ExposureState_EXPOSURE_STATE_FAILED,
}

func ExposureStatus(s domain.ExposureStatus) *dislacksov1.ExposureStatus {
	return &dislacksov1.ExposureStatus{
		Kind:     exposureKinds[s.Kind],
		State:    exposureStates[s.State],
		Url:      s.URL,
		LoginUrl: s.LoginURL,
		Problem:  s.Problem,
		Detail:   s.Detail,
		Stage:    s.Stage,
		SinceMs:  s.Since.UnixMilli(),
	}
}

var turnKinds = map[domain.TURNKind]dislacksov1.TurnKind{
	domain.TURNNone:       dislacksov1.TurnKind_TURN_KIND_NONE,
	domain.TURNCloudflare: dislacksov1.TurnKind_TURN_KIND_CLOUDFLARE,
	domain.TURNStatic:     dislacksov1.TurnKind_TURN_KIND_STATIC,
	domain.TURNCoturnREST: dislacksov1.TurnKind_TURN_KIND_COTURN_REST,
}

func TURNKind(k domain.TURNKind) dislacksov1.TurnKind { return turnKinds[k] }

func TURNKindFromProto(k dislacksov1.TurnKind) domain.TURNKind { return reverse(turnKinds, k) }

var natTypes = map[domain.NATType]dislacksov1.NatType{
	domain.NATNoInternet:          dislacksov1.NatType_NAT_TYPE_NO_INTERNET,
	domain.NATOpen:                dislacksov1.NatType_NAT_TYPE_OPEN,
	domain.NATPortPreserving:      dislacksov1.NatType_NAT_TYPE_PORT_PRESERVING,
	domain.NATEndpointIndependent: dislacksov1.NatType_NAT_TYPE_ENDPOINT_INDEPENDENT,
	domain.NATSymmetric:           dislacksov1.NatType_NAT_TYPE_SYMMETRIC,
}

var hostRatings = map[domain.HostRating]dislacksov1.HostRating{
	domain.RatingExcellent:      dislacksov1.HostRating_HOST_RATING_EXCELLENT,
	domain.RatingGood:           dislacksov1.HostRating_HOST_RATING_GOOD,
	domain.RatingRelayDependent: dislacksov1.HostRating_HOST_RATING_RELAY_DEPENDENT,
	domain.RatingOffline:        dislacksov1.HostRating_HOST_RATING_OFFLINE,
}

func SelfTest(t domain.SelfTest) *dislacksov1.SelfTestReport {
	return &dislacksov1.SelfTestReport{
		RanAtMs:    t.RanAt.UnixMilli(),
		MediaUp:    t.MediaUp,
		Nat:        natTypes[t.NAT],
		PublicIpv4: t.PublicIPv4,
		Ipv6:       t.IPv6,
		PublicIpv6: t.PublicIPv6,
		Mapping: &dislacksov1.PortMapping{
			Protocol: t.Mapping.Protocol, External: t.Mapping.External, Ok: t.Mapping.OK, Detail: t.Mapping.Detail,
		},
		Rating:          hostRatings[t.Rating],
		Advice:          t.Advice,
		RelayConfigured: t.RelayConfigured,
	}
}
