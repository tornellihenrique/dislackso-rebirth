// Package settings owns the instance settings: cached reads, validated
// updates, and the SMTP password kept in the secret store.
package settings

import (
	"context"
	"fmt"
	"sync"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

// SMTPPasswordSecret is the secret-store key of the SMTP password.
const SMTPPasswordSecret = "smtp.password"

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
	SMTPPassword      string
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
	if err := domain.Violations(checks...); err != nil {
		return domain.InstanceSettings{}, err
	}
	if p.MarkSetupComplete && next.SetupCompletedAt == nil {
		now := s.clock.Now()
		next.SetupCompletedAt = &now
	}

	if p.SMTPPassword != "" {
		if err := s.secrets.SetSecret(ctx, SMTPPasswordSecret, p.SMTPPassword); err != nil {
			return domain.InstanceSettings{}, fmt.Errorf("store smtp password: %w", err)
		}
	}
	if err := s.repo.SaveSettings(ctx, next); err != nil {
		return domain.InstanceSettings{}, fmt.Errorf("save settings: %w", err)
	}
	s.cache = &next
	return next, nil
}
