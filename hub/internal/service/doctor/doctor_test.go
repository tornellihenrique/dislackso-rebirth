package doctor

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

var (
	lan    = netip.MustParseAddr("192.168.1.20")
	public = netip.MustParseAddr("203.0.113.9")
)

func TestClassify(t *testing.T) {
	ap := netip.AddrPortFrom
	tests := []struct {
		name   string
		mapped []netip.AddrPort
		local  []netip.Addr
		want   domain.NATType
	}{
		{"no answer", []netip.AddrPort{{}, {}}, []netip.Addr{lan}, domain.NATNoInternet},
		{"own public address", []netip.AddrPort{ap(public, 7882), ap(public, 7882)}, []netip.Addr{lan, public}, domain.NATOpen},
		{"same port outside", []netip.AddrPort{ap(public, 7882), ap(public, 7882)}, []netip.Addr{lan}, domain.NATPortPreserving},
		{"one server answered", []netip.AddrPort{{}, ap(public, 7882)}, []netip.Addr{lan}, domain.NATPortPreserving},
		{"other port, same for both", []netip.AddrPort{ap(public, 40001), ap(public, 40001)}, []netip.Addr{lan}, domain.NATEndpointIndependent},
		{"port per destination", []netip.AddrPort{ap(public, 40001), ap(public, 40002)}, []netip.Addr{lan}, domain.NATSymmetric},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(7882, tt.mapped, tt.local); got != tt.want {
				t.Errorf("Classify = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRate(t *testing.T) {
	running := domain.ExposureStatus{Kind: domain.ExposureTailscaleFunnel, State: domain.ExposureRunning}
	tests := []struct {
		name       string
		test       domain.SelfTest
		exposure   domain.ExposureStatus
		want       domain.HostRating
		wantAdvice []string
	}{
		{
			name: "forwarded port", exposure: running,
			test: domain.SelfTest{MediaUp: true, NAT: domain.NATSymmetric, Mapping: domain.PortMapping{OK: true}},
			want: domain.RatingExcellent,
		},
		{
			name: "IPv6", exposure: running,
			test:       domain.SelfTest{MediaUp: true, NAT: domain.NATEndpointIndependent, IPv6: true},
			want:       domain.RatingExcellent,
			wantAdvice: []string{domain.AdviceEnableUPnP},
		},
		{
			name: "port preserving", exposure: running,
			test:       domain.SelfTest{MediaUp: true, NAT: domain.NATPortPreserving},
			want:       domain.RatingGood,
			wantAdvice: []string{domain.AdviceEnableUPnP},
		},
		{
			name: "endpoint independent", exposure: running,
			test:       domain.SelfTest{MediaUp: true, NAT: domain.NATEndpointIndependent, RelayConfigured: true},
			want:       domain.RatingGood,
			wantAdvice: []string{domain.AdviceEnableUPnP},
		},
		{
			name: "symmetric without relay", exposure: running,
			test:       domain.SelfTest{MediaUp: true, NAT: domain.NATSymmetric},
			want:       domain.RatingRelayDependent,
			wantAdvice: []string{domain.AdviceEnableUPnP, domain.AdviceRelayNeeded},
		},
		{
			name: "symmetric with relay", exposure: running,
			test:       domain.SelfTest{MediaUp: true, NAT: domain.NATSymmetric, RelayConfigured: true},
			want:       domain.RatingRelayDependent,
			wantAdvice: []string{domain.AdviceEnableUPnP, domain.AdviceRelayQuota},
		},
		{
			name:       "offline, local only, media down",
			exposure:   domain.ExposureStatus{Kind: domain.ExposureLocal, State: domain.ExposureOff},
			test:       domain.SelfTest{NAT: domain.NATNoInternet},
			want:       domain.RatingOffline,
			wantAdvice: []string{domain.AdviceMediaDown, domain.AdviceExposureOff, domain.AdviceOffline},
		},
		{
			name:       "exposure failing",
			exposure:   domain.ExposureStatus{Kind: domain.ExposureCloudflareQuick, State: domain.ExposureFailed},
			test:       domain.SelfTest{MediaUp: true, NAT: domain.NATOpen},
			want:       domain.RatingExcellent,
			wantAdvice: []string{domain.AdviceExposureDown},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, advice := Rate(tt.test, tt.exposure)
			if got != tt.want {
				t.Errorf("rating = %s, want %s", got, tt.want)
			}
			if diff := cmp.Diff(tt.wantAdvice, advice); diff != "" {
				t.Errorf("advice (-want +got):\n%s", diff)
			}
		})
	}
}

type fakeMapper struct {
	mu       sync.Mutex
	err      error
	maps     int
	unmapped bool
}

func (m *fakeMapper) Map(_ context.Context, udp, tcp int) (ports.MappedPorts, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maps++
	if m.err != nil {
		return ports.MappedPorts{}, m.err
	}
	return ports.MappedPorts{
		Protocol: "upnp", External: netip.AddrPortFrom(public, uint16(udp)), Lifetime: time.Hour,
		TCP: tcp != 0,
	}, nil
}

func (m *fakeMapper) Unmap(context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unmapped = true
}

type readyMedia bool

func (r readyMedia) Ready() bool { return bool(r) }

type exposureStub struct{ st domain.ExposureStatus }

func (e exposureStub) Status() domain.ExposureStatus { return e.st }

type settingsStub struct{ st domain.InstanceSettings }

func (s settingsStub) Get(context.Context) (domain.InstanceSettings, error) { return s.st, nil }

func newService(mapper ports.PortMapper) *Service {
	return New(Deps{
		STUN: func(_ context.Context, _ net.PacketConn, network string) []netip.AddrPort {
			if network == "udp6" {
				return nil
			}
			return []netip.AddrPort{netip.AddrPortFrom(public, 40001), netip.AddrPortFrom(public, 40001)}
		},
		LocalAddrs: func() []netip.Addr { return []netip.Addr{lan} },
		Mapper:     mapper,
		Media:      readyMedia(true),
		Exposure:   exposureStub{domain.ExposureStatus{Kind: domain.ExposureTailscaleFunnel, State: domain.ExposureRunning}},
		Settings:   settingsStub{domain.DefaultInstanceSettings()},
		Clock:      fakes.NewClock(),
		Logger:     slog.New(slog.DiscardHandler),
	})
}

func TestRunForwardsPortsThenTests(t *testing.T) {
	mapper := &fakeMapper{}
	s := newService(mapper)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Run(ctx, 7882, 7881)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for s.Last() == nil {
		if time.Now().After(deadline) {
			t.Fatal("no self-test after the first forwarding attempt")
		}
		time.Sleep(10 * time.Millisecond)
	}
	got := s.Last()
	if !got.Mapping.OK || got.Mapping.External != "203.0.113.9:7882" || got.Rating != domain.RatingExcellent {
		t.Errorf("self-test = %+v, want the forwarded port and an excellent rating", got)
	}
	cancel()
	<-done
	if !mapper.unmapped {
		t.Error("forwarding not removed on stop")
	}
}

func TestRunRecordsWhyForwardingFailed(t *testing.T) {
	mapper := &fakeMapper{err: errors.New("no router answered")}
	s := newService(mapper)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Run(ctx, 7882, 7881)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for s.Last() == nil {
		if time.Now().After(deadline) {
			t.Fatal("no self-test")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := s.Last(); got.Mapping.OK || got.Mapping.Detail != "no router answered" || got.NAT != domain.NATEndpointIndependent {
		t.Errorf("self-test = %+v", got)
	}
	cancel()
	<-done
	if mapper.unmapped {
		t.Error("unmapped a forwarding that never existed")
	}
}

func TestRunSelfTestNeedsAnInstanceAdmin(t *testing.T) {
	s := newService(&fakeMapper{})
	if _, err := s.RunSelfTest(t.Context(), domain.Actor{UserID: 2}); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("err = %v, want forbidden", err)
	}
}
