package mail

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

type staticConfig struct {
	settings domain.SMTPSettings
	password string
}

func (c staticConfig) SMTPConfig(context.Context) (domain.SMTPSettings, string, error) {
	return c.settings, c.password, nil
}

type recordingSMTP struct {
	sent     []ports.Mail
	password string
}

func (r *recordingSMTP) SendWith(_ context.Context, _ domain.SMTPSettings, password string, m ports.Mail) error {
	r.sent = append(r.sent, m)
	r.password = password
	return nil
}

func TestRouter(t *testing.T) {
	msg := ports.Mail{To: "a@example.com", Subject: "Hi", Text: "body"}

	tests := []struct {
		name          string
		smtpEnabled   bool
		devLog        bool
		wantAvailable bool
		wantSMTP      bool
		wantLogged    bool
		wantErr       error
	}{
		{name: "smtp enabled", smtpEnabled: true, wantAvailable: true, wantSMTP: true},
		{name: "smtp wins over the dev log", smtpEnabled: true, devLog: true, wantAvailable: true, wantSMTP: true},
		{name: "dev log only", devLog: true, wantAvailable: true, wantLogged: true},
		{name: "nothing configured", wantErr: ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			smtp := &recordingSMTP{}
			r := &Router{
				Config: staticConfig{settings: domain.SMTPSettings{Enabled: tt.smtpEnabled}, password: "pw"},
				SMTP:   smtp,
			}
			if tt.devLog {
				r.DevLog = &Log{Logger: slog.New(slog.NewTextHandler(&logs, nil))}
			}

			if got := r.Available(t.Context()); got != tt.wantAvailable {
				t.Errorf("Available = %v, want %v", got, tt.wantAvailable)
			}
			err := r.Send(t.Context(), msg)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Send err = %v, want %v", err, tt.wantErr)
			}
			if got := len(smtp.sent) == 1; got != tt.wantSMTP {
				t.Errorf("sent via smtp = %v, want %v", got, tt.wantSMTP)
			}
			if tt.wantSMTP && smtp.password != "pw" {
				t.Errorf("smtp password = %q, want the stored one", smtp.password)
			}
			if got := strings.Contains(logs.String(), "subject=Hi"); got != tt.wantLogged {
				t.Errorf("logged = %v, want %v (log: %s)", got, tt.wantLogged, logs.String())
			}
		})
	}
}
