// Package connectivity is the admin panel's view of how reachable the hub
// is: exposure, relays, the self-test and the host's conveniences together.
package connectivity

import (
	"context"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/exposure"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/relay"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/settings"
)

const (
	relayTestTTL = 5 * time.Minute
	// Relay tests run in rooms the voice service ignores.
	relayTestRoomPrefix = "diag_"
)

type Exposure interface {
	Status() domain.ExposureStatus
	Apply(ctx context.Context, actor domain.Actor, p exposure.Patch) (domain.InstanceSettings, error)
}

type Relay interface {
	Usage(ctx context.Context) (domain.TURNUsage, error)
	Apply(ctx context.Context, actor domain.Actor, p relay.Patch) (domain.InstanceSettings, error)
	Issue(ctx context.Context, userID domain.ID, ttl time.Duration) ([]ports.IceServer, error)
}

type Doctor interface {
	Last() *domain.SelfTest
	RunSelfTest(ctx context.Context, actor domain.Actor) (domain.SelfTest, error)
}

type Host interface {
	Options(ctx context.Context) (domain.HostOptions, error)
	Set(ctx context.Context, actor domain.Actor, client domain.ClientInfo, keepAwake, runAtLogin *bool) (domain.HostOptions, error)
	AddFirewallRule(ctx context.Context, actor domain.Actor, client domain.ClientInfo) error
}

type Settings interface {
	Get(ctx context.Context) (domain.InstanceSettings, error)
	SecretSet(ctx context.Context, key string) (bool, error)
}

type Media interface {
	URL() string
	JoinToken(j ports.MediaJoin) (string, error)
}

type Deps struct {
	Settings Settings
	Exposure Exposure
	Relay    Relay
	Doctor   Doctor
	Host     Host
	Media    Media
	Tokens   ports.TokenGen
}

// View is everything the Connectivity page shows.
type View struct {
	Exposure       domain.ExposureSettings
	TunnelTokenSet bool
	DNSTokenSet    bool
	Status         domain.ExposureStatus
	TURN           domain.TURNSettings
	TURNSecretSet  bool
	BudgetGB       int
	Usage          domain.TURNUsage
	SelfTest       *domain.SelfTest
	Host           domain.HostOptions
}

type Service struct {
	d      Deps
	policy domain.Policy
}

func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) Get(ctx context.Context, actor domain.Actor) (View, error) {
	if err := s.policy.ManageInstance(actor); err != nil {
		return View{}, err
	}
	return s.view(ctx)
}

func (s *Service) view(ctx context.Context) (View, error) {
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return View{}, err
	}
	v := View{
		Exposure: st.Exposure, Status: s.d.Exposure.Status(),
		TURN: st.TURN, BudgetGB: st.Limits.TURNMonthlyBudgetGB,
		SelfTest: s.d.Doctor.Last(),
	}
	if v.TunnelTokenSet, err = s.d.Settings.SecretSet(ctx, settings.TunnelTokenSecret); err != nil {
		return View{}, err
	}
	if v.DNSTokenSet, err = s.d.Settings.SecretSet(ctx, settings.DNSTokenSecret); err != nil {
		return View{}, err
	}
	if v.TURNSecretSet, err = s.d.Settings.SecretSet(ctx, settings.TURNSecret); err != nil {
		return View{}, err
	}
	if v.Usage, err = s.d.Relay.Usage(ctx); err != nil {
		return View{}, err
	}
	if v.Host, err = s.d.Host.Options(ctx); err != nil {
		return View{}, err
	}
	return v, nil
}

func (s *Service) SetExposure(ctx context.Context, actor domain.Actor, p exposure.Patch) (View, error) {
	if _, err := s.d.Exposure.Apply(ctx, actor, p); err != nil {
		return View{}, err
	}
	return s.view(ctx)
}

func (s *Service) SetTURN(ctx context.Context, actor domain.Actor, p relay.Patch) (View, error) {
	if _, err := s.d.Relay.Apply(ctx, actor, p); err != nil {
		return View{}, err
	}
	return s.view(ctx)
}

func (s *Service) RunSelfTest(ctx context.Context, actor domain.Actor) (domain.SelfTest, error) {
	return s.d.Doctor.RunSelfTest(ctx, actor)
}

// RelayTest is a token for a throwaway media room with relay servers only,
// so the admin's browser can prove that relayed clients connect.
type RelayTest struct {
	URL   string
	Token string
	ICE   []ports.IceServer
}

func (s *Service) RelayTest(ctx context.Context, actor domain.Actor) (RelayTest, error) {
	if err := s.policy.ManageInstance(actor); err != nil {
		return RelayTest{}, err
	}
	ice, err := s.d.Relay.Issue(ctx, actor.UserID, relayTestTTL)
	if err != nil {
		return RelayTest{}, err
	}
	if len(ice) == 0 {
		return RelayTest{}, domain.Precondition(domain.ReasonRelayNotConfigured)
	}
	suffix, err := s.d.Tokens.NewToken()
	if err != nil {
		return RelayTest{}, err
	}
	token, err := s.d.Media.JoinToken(ports.MediaJoin{
		Room:        relayTestRoomPrefix + suffix,
		Identity:    "diag-" + actor.UserID.String(),
		Permissions: domain.VoicePermissions{Publish: []domain.MediaSource{domain.SourceMicrophone}, Subscribe: true},
		TTL:         relayTestTTL,
	})
	if err != nil {
		return RelayTest{}, err
	}
	return RelayTest{URL: s.d.Media.URL(), Token: token, ICE: ice}, nil
}

func (s *Service) SetHost(ctx context.Context, actor domain.Actor, client domain.ClientInfo, keepAwake, runAtLogin *bool) (domain.HostOptions, error) {
	return s.d.Host.Set(ctx, actor, client, keepAwake, runAtLogin)
}

func (s *Service) AddFirewallRule(ctx context.Context, actor domain.Actor, client domain.ClientInfo) error {
	return s.d.Host.AddFirewallRule(ctx, actor, client)
}
