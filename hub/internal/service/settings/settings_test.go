package settings

import (
	"errors"
	"testing"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
)

func TestUpdate(t *testing.T) {
	store, secrets := fakes.NewStore(), &fakes.Secrets{}
	svc := New(store, secrets, fakes.NewClock())
	ctx := t.Context()

	smtp := domain.SMTPSettings{Enabled: true, Host: "smtp.example.com", Port: 587, Security: domain.SMTPStartTLS, FromAddress: "hub@example.com"}
	got, err := svc.Update(ctx, Patch{
		Name:              new("Team"),
		PublicURL:         new(" https://hub.example.com/ "),
		RegistrationMode:  new(domain.RegistrationOpen),
		SMTP:              &smtp,
		SMTPPassword:      "app-password",
		MarkSetupComplete: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Team" || got.PublicURL != "https://hub.example.com" || got.RegistrationMode != domain.RegistrationOpen || !got.SetupComplete() {
		t.Errorf("settings = %+v", got)
	}
	s, pw, err := svc.SMTPConfig(ctx)
	if err != nil || s.Host != "smtp.example.com" || pw != "app-password" {
		t.Errorf("SMTPConfig = %+v, %q, %v", s, pw, err)
	}

	if _, err := svc.Update(ctx, Patch{SMTP: &smtp}); err != nil {
		t.Fatal(err)
	}
	if _, pw, _ := svc.SMTPConfig(ctx); pw != "app-password" {
		t.Error("an empty password in a patch must keep the stored one")
	}

	svc.Invalidate()
	reloaded, err := svc.Get(ctx)
	if err != nil || reloaded.Name != "Team" {
		t.Errorf("after reload: %+v, %v", reloaded, err)
	}
}

func TestUpdateRejectsInvalid(t *testing.T) {
	svc := New(fakes.NewStore(), &fakes.Secrets{}, fakes.NewClock())
	tests := []struct {
		name  string
		patch Patch
		field string
	}{
		{"empty name", Patch{Name: new("")}, "name"},
		{"relative public url", Patch{PublicURL: new("hub.example.com")}, "public_url"},
		{"public url with query", Patch{PublicURL: new("https://hub.example.com/?a=1")}, "public_url"},
		{"unknown registration mode", Patch{RegistrationMode: new(domain.RegistrationMode("maybe"))}, "registration_mode"},
		{"enabled smtp without host", Patch{SMTP: &domain.SMTPSettings{Enabled: true, Port: 587, Security: domain.SMTPTLS, FromAddress: "a@example.com"}}, "smtp.host"},
		{"enabled smtp with bad sender", Patch{SMTP: &domain.SMTPSettings{Enabled: true, Host: "h", Port: 587, Security: domain.SMTPTLS, FromAddress: "nope"}}, "smtp.from_address"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Update(t.Context(), tt.patch)
			de, ok := domain.AsError(err)
			if !errors.Is(err, domain.ErrInvalid) || !ok || de.Fields[0].Field != tt.field {
				t.Fatalf("err = %v, want invalid %s", err, tt.field)
			}
		})
	}
	if _, err := svc.Update(t.Context(), Patch{SMTP: &domain.SMTPSettings{Enabled: false}}); err != nil {
		t.Errorf("a disabled smtp config may be incomplete: %v", err)
	}
}
