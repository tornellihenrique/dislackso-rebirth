// Package mail delivers email through SMTP, or to the log in development.
package mail

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	gomail "github.com/wneessen/go-mail"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

const smtpTimeout = 20 * time.Second

// SMTP sends through a mail server. It implements ports.MailTester.
type SMTP struct{}

func (SMTP) SendWith(ctx context.Context, s domain.SMTPSettings, password string, m ports.Mail) error {
	msg := gomail.NewMsg()
	if err := msg.FromFormat(s.FromName, s.FromAddress); err != nil {
		return fmt.Errorf("set sender: %w", err)
	}
	if err := msg.To(m.To); err != nil {
		return fmt.Errorf("set recipient: %w", err)
	}
	msg.Subject(m.Subject)
	msg.SetBodyString(gomail.TypeTextPlain, m.Text)
	if m.HTML != "" {
		msg.AddAlternativeString(gomail.TypeTextHTML, m.HTML)
	}

	var opts []gomail.Option
	switch s.Security {
	case domain.SMTPTLS:
		opts = append(opts, gomail.WithSSL())
	case domain.SMTPNone:
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	case domain.SMTPStartTLS:
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSMandatory))
	}
	// The port goes after the security option, which may pick its own default.
	opts = append(opts, gomail.WithPort(s.Port), gomail.WithTimeout(smtpTimeout))
	if s.Username != "" {
		opts = append(opts,
			gomail.WithSMTPAuth(gomail.SMTPAuthAutoDiscover),
			gomail.WithUsername(s.Username),
			gomail.WithPassword(password),
		)
	}
	client, err := gomail.NewClient(s.Host, opts...)
	if err != nil {
		return fmt.Errorf("configure smtp client: %w", err)
	}
	if err := client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("send via %s:%d: %w", s.Host, s.Port, err)
	}
	return nil
}

// Log writes messages to the logger instead of sending them; for development.
type Log struct {
	Logger *slog.Logger
}

func (l Log) Send(ctx context.Context, m ports.Mail) error {
	l.Logger.InfoContext(ctx, "email (not sent: development mail log)",
		slog.String("to", m.To),
		slog.String("subject", m.Subject),
		slog.String("body", m.Text),
	)
	return nil
}

// ConfigSource returns the current SMTP settings and password.
type ConfigSource interface {
	SMTPConfig(ctx context.Context) (domain.SMTPSettings, string, error)
}

// Router implements ports.Mailer: SMTP when the instance has it enabled,
// otherwise the development log when that is on.
type Router struct {
	Config ConfigSource
	SMTP   ports.MailTester
	// DevLog, when non-nil, receives mail while SMTP is disabled.
	DevLog *Log
}

var ErrUnavailable = errors.New("email delivery is not configured")

func (r *Router) Available(ctx context.Context) bool {
	if r.DevLog != nil {
		return true
	}
	s, _, err := r.Config.SMTPConfig(ctx)
	return err == nil && s.Enabled
}

func (r *Router) Send(ctx context.Context, m ports.Mail) error {
	s, password, err := r.Config.SMTPConfig(ctx)
	if err != nil {
		return err
	}
	if s.Enabled {
		return r.SMTP.SendWith(ctx, s, password, m)
	}
	if r.DevLog != nil {
		return r.DevLog.Send(ctx, m)
	}
	return ErrUnavailable
}
