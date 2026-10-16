// Package doctor checks how reachable the host is for voice (ARCHITECTURE
// §4.4): what its router does to the media port, whether it has IPv6, whether
// the router forwards the media ports, and what that means for people
// outside. It also keeps that port forwarding up.
package doctor

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

const (
	// Mappings are renewed well before they lapse; permanent ones are
	// re-added in case the router restarted.
	renewEvery = 30 * time.Minute
	retryEvery = 10 * time.Minute
	unmapGrace = 3 * time.Second
	// The first self-test waits this long at most for the media server.
	mediaStartWait = 30 * time.Second
	mediaPoll      = 500 * time.Millisecond
)

// STUN asks STUN servers for conn's public address; an invalid address
// stands for a server that did not answer.
type STUN func(ctx context.Context, conn net.PacketConn, network string) []netip.AddrPort

type MediaChecker interface{ Ready() bool }

type ExposureStatus interface {
	Status() domain.ExposureStatus
}

type Settings interface {
	Get(ctx context.Context) (domain.InstanceSettings, error)
}

type Deps struct {
	STUN STUN
	// LocalAddrs lists this machine's interface addresses.
	LocalAddrs func() []netip.Addr
	Mapper     ports.PortMapper
	Media      MediaChecker
	Exposure   ExposureStatus
	Settings   Settings
	Clock      ports.Clock
	Logger     *slog.Logger
}

// Probe is what STUN tells about the host's addresses.
type Probe struct {
	NAT        domain.NATType
	PublicIPv4 netip.Addr
	PublicIPv6 netip.Addr
}

type Service struct {
	d      Deps
	policy domain.Policy

	mu      sync.Mutex
	mapping domain.PortMapping
	last    *domain.SelfTest
}

func New(d Deps) *Service { return &Service{d: d} }

// Probe classifies the router's handling of UDP from port; zero picks a free
// port. Called on the media port before the media server binds it, it shows
// whether that very port keeps its number outside.
func (s *Service) Probe(ctx context.Context, port int) Probe {
	var lc net.ListenConfig
	conn, err := lc.ListenPacket(ctx, "udp4", ":"+strconv.Itoa(port))
	if err != nil && port != 0 {
		conn, err = lc.ListenPacket(ctx, "udp4", ":0")
	}
	if err != nil {
		return Probe{NAT: domain.NATNoInternet}
	}
	local, _ := netip.ParseAddrPort(conn.LocalAddr().String())
	mapped := s.d.STUN(ctx, conn, "udp4")
	_ = conn.Close()
	addrs := s.d.LocalAddrs()
	p := Probe{NAT: Classify(local.Port(), mapped, addrs)}
	for _, m := range mapped {
		if m.IsValid() {
			p.PublicIPv4 = m.Addr()
			break
		}
	}
	if slices.ContainsFunc(addrs, globalIPv6) {
		if conn6, err := lc.ListenPacket(ctx, "udp6", "[::]:0"); err == nil {
			for _, m := range s.d.STUN(ctx, conn6, "udp6") {
				if m.IsValid() {
					p.PublicIPv6 = m.Addr()
					break
				}
			}
			_ = conn6.Close()
		}
	}
	return p
}

// Classify tells the NAT type from the addresses two STUN servers saw.
func Classify(localPort uint16, mapped []netip.AddrPort, local []netip.Addr) domain.NATType {
	var seen []netip.AddrPort
	for _, m := range mapped {
		if m.IsValid() {
			seen = append(seen, m)
		}
	}
	switch {
	case len(seen) == 0:
		return domain.NATNoInternet
	case slices.Contains(local, seen[0].Addr()):
		return domain.NATOpen
	}
	for _, m := range seen[1:] {
		if m != seen[0] {
			return domain.NATSymmetric
		}
	}
	if seen[0].Port() == localPort {
		return domain.NATPortPreserving
	}
	return domain.NATEndpointIndependent
}

// Rate turns the findings into the host rating and the advice to show.
func Rate(t domain.SelfTest, exposure domain.ExposureStatus) (domain.HostRating, []string) {
	var advice []string
	if !t.MediaUp {
		advice = append(advice, domain.AdviceMediaDown)
	}
	switch {
	case exposure.Kind == domain.ExposureLocal:
		advice = append(advice, domain.AdviceExposureOff)
	case exposure.State != domain.ExposureRunning:
		advice = append(advice, domain.AdviceExposureDown)
	}
	if t.NAT == domain.NATNoInternet {
		return domain.RatingOffline, append(advice, domain.AdviceOffline)
	}
	if !t.Mapping.OK && t.NAT != domain.NATOpen {
		advice = append(advice, domain.AdviceEnableUPnP)
	}
	switch {
	case t.Mapping.OK || t.NAT == domain.NATOpen || t.IPv6:
		return domain.RatingExcellent, advice
	case t.NAT == domain.NATPortPreserving:
		return domain.RatingGood, advice
	}
	if t.RelayConfigured {
		return domain.RatingRelayDependent, append(advice, domain.AdviceRelayQuota)
	}
	return domain.RatingRelayDependent, append(advice, domain.AdviceRelayNeeded)
}

// SelfTest checks everything now and keeps the result for the admin panel.
func (s *Service) SelfTest(ctx context.Context) domain.SelfTest {
	p := s.Probe(ctx, 0)
	t := domain.SelfTest{
		RanAt:   s.d.Clock.Now(),
		MediaUp: s.d.Media.Ready(),
		NAT:     p.NAT,
		IPv6:    p.PublicIPv6.IsValid(),
	}
	if p.PublicIPv4.IsValid() {
		t.PublicIPv4 = p.PublicIPv4.String()
	}
	if t.IPv6 {
		t.PublicIPv6 = p.PublicIPv6.String()
	}
	s.mu.Lock()
	t.Mapping = s.mapping
	s.mu.Unlock()
	if st, err := s.d.Settings.Get(ctx); err == nil {
		t.RelayConfigured = st.TURN.Kind != domain.TURNNone
	}
	t.Rating, t.Advice = Rate(t, s.d.Exposure.Status())
	s.mu.Lock()
	s.last = &t
	s.mu.Unlock()
	s.d.Logger.InfoContext(ctx, "connectivity self-test",
		slog.String("rating", string(t.Rating)), slog.String("nat", string(t.NAT)),
		slog.Bool("ipv6", t.IPv6), slog.Bool("port_mapping", t.Mapping.OK))
	return t
}

// RunSelfTest is SelfTest for an instance admin.
func (s *Service) RunSelfTest(ctx context.Context, actor domain.Actor) (domain.SelfTest, error) {
	if err := s.policy.ManageInstance(actor); err != nil {
		return domain.SelfTest{}, err
	}
	return s.SelfTest(ctx), nil
}

// Last is the latest self-test, if one ran.
func (s *Service) Last() *domain.SelfTest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}

// Run keeps the media ports forwarded on the router until ctx ends, then
// removes the forwarding. The first self-test runs once forwarding was tried
// and the media server had time to start; udp 0 means no media server.
func (s *Service) Run(ctx context.Context, udp, tcp int) {
	if udp == 0 {
		s.SelfTest(ctx)
		return
	}
	tried := make(chan struct{})
	go func() {
		select {
		case <-tried:
		case <-ctx.Done():
			return
		}
		s.awaitMedia(ctx)
		if ctx.Err() == nil {
			s.SelfTest(ctx)
		}
	}()
	s.keepMapped(ctx, udp, tcp, tried)
}

func (s *Service) awaitMedia(ctx context.Context) {
	deadline := time.After(mediaStartWait)
	tick := time.NewTicker(mediaPoll)
	defer tick.Stop()
	for !s.d.Media.Ready() {
		select {
		case <-ctx.Done():
			return
		case <-deadline:
			return
		case <-tick.C:
		}
	}
}

func (s *Service) keepMapped(ctx context.Context, udp, tcp int, tried chan<- struct{}) {
	var logged string
	for {
		m, err := s.d.Mapper.Map(ctx, udp, tcp)
		wait := retryEvery
		s.mu.Lock()
		if err != nil {
			s.mapping = domain.PortMapping{Detail: err.Error()}
		} else {
			s.mapping = domain.PortMapping{Protocol: m.Protocol, External: m.External.String(), OK: true}
			wait = renewEvery
			if m.Lifetime > 0 && m.Lifetime/2 < wait {
				wait = m.Lifetime / 2
			}
		}
		s.mu.Unlock()
		if tried != nil {
			close(tried)
			tried = nil
		}
		// Renewals repeat the same outcome; only changes are logged.
		outcome := "on " + m.External.String()
		if err != nil {
			outcome = err.Error()
		}
		if outcome != logged && ctx.Err() == nil {
			logged = outcome
			if err != nil {
				s.d.Logger.InfoContext(ctx, "router port forwarding unavailable", slog.Any("error", err))
			} else {
				s.d.Logger.InfoContext(ctx, "router port forwarding on",
					slog.String("protocol", m.Protocol), slog.String("external", m.External.String()))
			}
		}
		select {
		case <-ctx.Done():
			if err == nil {
				unmapCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), unmapGrace)
				s.d.Mapper.Unmap(unmapCtx)
				cancel()
			}
			return
		case <-time.After(wait):
		}
	}
}

func globalIPv6(a netip.Addr) bool {
	return a.Is6() && !a.Is4In6() && a.IsGlobalUnicast() && !a.IsPrivate()
}
