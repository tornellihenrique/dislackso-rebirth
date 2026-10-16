package domain

import "time"

// NATType is how the host's router treats the media server's traffic
// (ARCHITECTURE §4.4).
type NATType string

const (
	NATNoInternet NATType = "no_internet"
	// NATOpen: the machine has a public address itself.
	NATOpen NATType = "open"
	// NATPortPreserving keeps the local port on the outside.
	NATPortPreserving NATType = "port_preserving"
	// NATEndpointIndependent keeps one outside port for every destination,
	// but not the local one.
	NATEndpointIndependent NATType = "endpoint_independent"
	// NATSymmetric uses a new outside port per destination.
	NATSymmetric NATType = "symmetric"
)

type HostRating string

const (
	RatingExcellent      HostRating = "excellent"
	RatingGood           HostRating = "good"
	RatingRelayDependent HostRating = "relay_dependent"
	RatingOffline        HostRating = "offline"
)

// Advice the self-test gives, as codes the admin panel explains.
const (
	AdviceOffline      = "offline_host"
	AdviceMediaDown    = "media_down"
	AdviceEnableUPnP   = "enable_upnp"
	AdviceRelayNeeded  = "relay_needed"
	AdviceRelayQuota   = "relay_quota"
	AdviceExposureOff  = "exposure_off"
	AdviceExposureDown = "exposure_down"
)

// PortMapping is the router forwarding the media ports, if any.
type PortMapping struct {
	// "nat-pmp" or "upnp"; empty when no router answered.
	Protocol string
	// External is the public address and port people reach the media server at.
	External string
	OK       bool
	Detail   string
}

// SelfTest is the connectivity self-test's result.
type SelfTest struct {
	RanAt           time.Time
	MediaUp         bool
	NAT             NATType
	PublicIPv4      string
	IPv6            bool
	PublicIPv6      string
	Mapping         PortMapping
	Rating          HostRating
	Advice          []string
	RelayConfigured bool
}

// HostOptions are the conveniences of the machine running the hub.
type HostOptions struct {
	KeepAwake           bool
	RunAtLogin          bool
	RunAtLoginSupported bool
	FirewallSupported   bool
}
