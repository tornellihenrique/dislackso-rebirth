// Package setup runs the first-run wizard: it only answers the host machine,
// requires the one-time token printed at startup, and closes for good once
// an admin exists.
package setup

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/auth"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/notify"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/settings"
)

var limitSetup = ports.Limit{Burst: 10, Every: 10 * time.Second}

type Settings interface {
	Get(ctx context.Context) (domain.InstanceSettings, error)
	Update(ctx context.Context, p settings.Patch) (domain.InstanceSettings, error)
	Invalidate()
}

type Sessions interface {
	StartSession(ctx context.Context, client domain.ClientInfo, user domain.User) (auth.SignedIn, error)
}

type Deps struct {
	Users    ports.UserRepo
	Tx       ports.TxRunner
	Hasher   ports.PasswordHasher
	IDs      ports.IDGen
	Clock    ports.Clock
	Limiter  ports.RateLimiter
	Settings Settings
	Sessions Sessions
	Mail     ports.MailTester
	// Token is the one-time setup token; the composition root generates it
	// (or takes it from configuration) and prints it.
	Token string
}

type Service struct {
	d Deps
	// mu serializes CompleteSetup so two concurrent requests cannot both
	// create an admin.
	mu sync.Mutex
}

func New(d Deps) *Service { return &Service{d: d} }

// Token is the one-time setup token, for the startup log.
func (s *Service) Token() string { return s.d.Token }

// Pending reports whether setup still has to run.
func (s *Service) Pending(ctx context.Context) (bool, error) {
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return false, err
	}
	return !st.SetupComplete(), nil
}

func (s *Service) CheckToken(ctx context.Context, client domain.ClientInfo, token string) error {
	if !client.IsLocal() {
		return domain.Forbidden(domain.ReasonNotLocal)
	}
	if ok, retry := s.d.Limiter.Allow("setup:ip:"+client.IP.String(), limitSetup); !ok {
		return domain.RateLimited(retry)
	}
	pending, err := s.Pending(ctx)
	if err != nil {
		return err
	}
	if !pending {
		return domain.Precondition(domain.ReasonSetupComplete)
	}
	if s.d.Token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(s.d.Token)) != 1 {
		return domain.Forbidden(domain.ReasonSetupTokenInvalid)
	}
	return nil
}

// SendTestEmail sends a test message through SMTP settings that are not saved yet.
func (s *Service) SendTestEmail(ctx context.Context, client domain.ClientInfo, token string, smtp domain.SMTPSettings, password, to string) error {
	if err := s.CheckToken(ctx, client, token); err != nil {
		return err
	}
	smtp.Enabled = true
	to = domain.NormalizeEmail(to)
	checks := domain.ValidateSMTP("smtp.", smtp)
	checks = append(checks, domain.ValidateEmail("to", to))
	if err := domain.Violations(checks...); err != nil {
		return err
	}
	m, err := notify.Render(notify.KindTest, client.Locale, to, notify.Data{Instance: s.instanceName(ctx)})
	if err != nil {
		return err
	}
	if err := s.d.Mail.SendWith(ctx, smtp, password, m); err != nil {
		return &domain.Error{Kind: domain.ErrFailedPrecondition, Reason: domain.ReasonEmailDeliveryFailed, Message: err.Error()}
	}
	return nil
}

type CompleteInput struct {
	Token         string
	InstanceName  string
	PublicURL     string
	AdminUsername string
	AdminEmail    string
	AdminPassword string
	// SMTP is nil, or disabled, to run without email.
	SMTP         *domain.SMTPSettings
	SMTPPassword string
}

// Complete creates the instance admin, saves the settings, closes setup and
// signs the admin in. Settings violations name the fields name, public_url
// and smtp.<field>; admin violations name admin_username, admin_email and
// admin_password.
func (s *Service) Complete(ctx context.Context, client domain.ClientInfo, in CompleteInput) (auth.SignedIn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.CheckToken(ctx, client, in.Token); err != nil {
		return auth.SignedIn{}, err
	}

	name := strings.TrimSpace(in.InstanceName)
	publicURL := domain.NormalizePublicURL(in.PublicURL)
	username := strings.TrimSpace(in.AdminUsername)
	email := domain.NormalizeEmail(in.AdminEmail)
	checks := []*domain.FieldViolation{
		domain.ValidateInstanceName("name", name),
		domain.ValidatePublicURL("public_url", publicURL),
		domain.ValidateUsername("admin_username", username),
		domain.ValidateEmail("admin_email", email),
		domain.ValidatePassword("admin_password", in.AdminPassword),
	}
	if in.SMTP != nil {
		checks = append(checks, domain.ValidateSMTP("smtp.", *in.SMTP)...)
	}
	if err := domain.Violations(checks...); err != nil {
		return auth.SignedIn{}, err
	}

	hash, err := s.d.Hasher.Hash(in.AdminPassword)
	if err != nil {
		return auth.SignedIn{}, fmt.Errorf("hash password: %w", err)
	}
	now := s.d.Clock.Now()
	admin := domain.User{
		ID:              s.d.IDs.NewID(),
		Username:        username,
		Email:           email,
		EmailVerifiedAt: &now,
		PasswordHash:    hash,
		DisplayName:     username,
		StatusPref:      domain.StatusOnline,
		IsInstanceAdmin: true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	patch := settings.Patch{Name: &name, PublicURL: &publicURL, MarkSetupComplete: true}
	if in.SMTP != nil {
		patch.SMTP = in.SMTP
		patch.SMTPPassword = in.SMTPPassword
	}

	var signed auth.SignedIn
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Users.CreateUser(ctx, admin); err != nil {
			return renameConflict(err)
		}
		if _, err := s.d.Settings.Update(ctx, patch); err != nil {
			return err
		}
		var err error
		signed, err = s.d.Sessions.StartSession(ctx, client, admin)
		return err
	})
	if err != nil {
		// Update may have cached settings from a transaction that rolled back.
		s.d.Settings.Invalidate()
		return auth.SignedIn{}, err
	}
	return signed, nil
}

func (s *Service) instanceName(ctx context.Context) string {
	return notify.InstanceName(ctx, s.d.Settings)
}

// renameConflict maps the users table's columns to this request's fields.
func renameConflict(err error) error {
	de, ok := domain.AsError(err)
	if !ok {
		return err
	}
	for i, f := range de.Fields {
		de.Fields[i].Field = "admin_" + f.Field
	}
	return de
}
