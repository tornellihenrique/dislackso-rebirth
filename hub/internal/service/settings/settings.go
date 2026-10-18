// Package settings owns the instance settings: cached reads, validated
// updates, and the secrets (SMTP password, tunnel token, dynamic DNS token,
// TURN secret) kept in the secret store.
package settings

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

// Secret-store keys.
const (
	SMTPPasswordSecret = "smtp.password"
	TunnelTokenSecret  = "exposure.tunnel_token"
	DNSTokenSecret     = "exposure.dns_token"
	TURNSecret         = "turn.secret"
)

type Service struct {
	repo    ports.SettingsRepo
	secrets ports.SecretStore
	clock   ports.Clock

	mu    sync.Mutex
	cache *domain.InstanceSettings
}

func New(repo ports.SettingsRepo, secrets ports.SecretStore, clock ports.Clock) *Service {
	return &Service{repo: repo, secrets: secrets, clock: clock}
}

// Get returns the current settings. They are cached: the hub is the only writer.
func (s *Service) Get(ctx context.Context) (domain.InstanceSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(ctx)
}

func (s *Service) load(ctx context.Context) (domain.InstanceSettings, error) {
	if s.cache != nil {
		return *s.cache, nil
	}
	loaded, err := s.repo.LoadSettings(ctx, domain.DefaultInstanceSettings())
	if err != nil {
		return domain.InstanceSettings{}, fmt.Errorf("load settings: %w", err)
	}
	s.cache = &loaded
	return loaded, nil
}

// Invalidate drops the cache, e.g. after a transaction that saved settings
// rolled back.
func (s *Service) Invalidate() {
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
}

// SMTPConfig returns the SMTP settings with the stored password.
func (s *Service) SMTPConfig(ctx context.Context) (domain.SMTPSettings, string, error) {
	cur, err := s.Get(ctx)
	if err != nil {
		return domain.SMTPSettings{}, "", err
	}
	password, _, err := s.secrets.Secret(ctx, SMTPPasswordSecret)
	if err != nil {
		return domain.SMTPSettings{}, "", fmt.Errorf("read smtp password: %w", err)
	}
	return cur.SMTP, password, nil
}

func (s *Service) SMTPPasswordSet(ctx context.Context) (bool, error) {
	_, ok, err := s.secrets.Secret(ctx, SMTPPasswordSecret)
	if err != nil {
		return false, fmt.Errorf("read smtp password: %w", err)
	}
	return ok, nil
}

// Patch lists the settings to change; nil fields keep their value.
type Patch struct {
	Name             *string
	PublicURL        *string
	RegistrationMode *domain.RegistrationMode
	MembersCanInvite *bool
	SMTP             *domain.SMTPSettings
	// SMTPPassword replaces the stored password when non-empty.
	SMTPPassword string
	Exposure     *domain.ExposureSettings
	// TunnelToken replaces the Cloudflare tunnel token when non-empty.
	TunnelToken string
	// DNSToken replaces the dynamic DNS token when non-empty.
	DNSToken string
	TURN     *domain.TURNSettings
	// TURNSecret replaces the relay provider's secret when non-empty.
	TURNSecret        string
	TURNBudgetGB      *int
	Host              *domain.HostSettings
	MarkSetupComplete bool
}

// Update validates and applies p. Violations name the fields as name,
// public_url, registration_mode and smtp.<field>.
func (s *Service) Update(ctx context.Context, p Patch) (domain.InstanceSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next, err := s.load(ctx)
	if err != nil {
		return domain.InstanceSettings{}, err
	}
	var checks []*domain.FieldViolation
	if p.Name != nil {
		next.Name = *p.Name
		checks = append(checks, domain.ValidateInstanceName("name", next.Name))
	}
	if p.PublicURL != nil {
		next.PublicURL = domain.NormalizePublicURL(*p.PublicURL)
		checks = append(checks, domain.ValidatePublicURL("public_url", next.PublicURL))
	}
	if p.RegistrationMode != nil {
		next.RegistrationMode = *p.RegistrationMode
		if !next.RegistrationMode.Valid() {
			checks = append(checks, &domain.FieldViolation{Field: "registration_mode", Reason: domain.ReasonFieldInvalidFormat})
		}
	}
	if p.MembersCanInvite != nil {
		next.MembersCanInvite = *p.MembersCanInvite
	}
	if p.SMTP != nil {
		next.SMTP = *p.SMTP
		checks = append(checks, domain.ValidateSMTP("smtp.", next.SMTP)...)
	}
	if p.Exposure != nil {
		next.Exposure = *p.Exposure
		next.Exposure.TailscaleHostname = strings.ToLower(strings.TrimSpace(next.Exposure.TailscaleHostname))
		next.Exposure.DirectDomain = strings.ToLower(strings.TrimSpace(next.Exposure.DirectDomain))
		next.Exposure.DynamicDNSName = strings.ToLower(strings.TrimSpace(next.Exposure.DynamicDNSName))
		next.Exposure.TunnelURL = domain.NormalizePublicURL(next.Exposure.TunnelURL)
		if next.Exposure.DynamicDNS == "" {
			next.Exposure.DynamicDNS = domain.DynamicDNSNone
		}
		tokenSet, err := s.secretSet(ctx, TunnelTokenSecret, p.TunnelToken)
		if err != nil {
			return domain.InstanceSettings{}, err
		}
		dnsTokenSet, err := s.secretSet(ctx, DNSTokenSecret, p.DNSToken)
		if err != nil {
			return domain.InstanceSettings{}, err
		}
		checks = append(checks, domain.ValidateExposure("exposure.", next.Exposure, tokenSet, dnsTokenSet)...)
	}
	if p.TURN != nil {
		next.TURN = *p.TURN
		secretSet, err := s.secretSet(ctx, TURNSecret, p.TURNSecret)
		if err != nil {
			return domain.InstanceSettings{}, err
		}
		checks = append(checks, domain.ValidateTURN("turn.", next.TURN, secretSet)...)
	}
	if p.TURNBudgetGB != nil {
		next.Limits.TURNMonthlyBudgetGB = *p.TURNBudgetGB
		if *p.TURNBudgetGB < 0 || *p.TURNBudgetGB > domain.TURNBudgetMaxGB {
			checks = append(checks, &domain.FieldViolation{Field: "turn.monthly_budget_gb", Reason: domain.ReasonFieldInvalid})
		}
	}
	if p.Host != nil {
		next.Host = *p.Host
	}
	if err := domain.Violations(checks...); err != nil {
		return domain.InstanceSettings{}, err
	}
	if p.MarkSetupComplete && next.SetupCompletedAt == nil {
		now := s.clock.Now()
		next.SetupCompletedAt = &now
	}

	for key, value := range map[string]string{
		SMTPPasswordSecret: p.SMTPPassword,
		TunnelTokenSecret:  p.TunnelToken,
		DNSTokenSecret:     p.DNSToken,
		TURNSecret:         p.TURNSecret,
	} {
		if value == "" {
			continue
		}
		if err := s.secrets.SetSecret(ctx, key, value); err != nil {
			return domain.InstanceSettings{}, fmt.Errorf("store %s: %w", key, err)
		}
	}
	if err := s.repo.SaveSettings(ctx, next); err != nil {
		return domain.InstanceSettings{}, fmt.Errorf("save settings: %w", err)
	}
	s.cache = &next
	return next, nil
}

// Secret returns a stored secret, or empty when there is none.
func (s *Service) Secret(ctx context.Context, key string) (string, error) {
	v, _, err := s.secrets.Secret(ctx, key)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", key, err)
	}
	return v, nil
}

// SecretSet reports whether a secret is stored, for write-only fields.
func (s *Service) SecretSet(ctx context.Context, key string) (bool, error) {
	return s.secretSet(ctx, key, "")
}

// secretSet reports whether a secret will be set after an update that may
// bring a new value.
func (s *Service) secretSet(ctx context.Context, key, incoming string) (bool, error) {
	if incoming != "" {
		return true, nil
	}
	_, ok, err := s.secrets.Secret(ctx, key)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", key, err)
	}
	return ok, nil
}
