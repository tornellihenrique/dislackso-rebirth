// Package fakes provides in-memory implementations of the ports for service
// tests.
package fakes

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

type Clock struct {
	mu sync.Mutex
	T  time.Time
}

func NewClock() *Clock { return &Clock{T: time.UnixMilli(1_790_000_000_000).UTC()} }

func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.T
}

func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	c.T = c.T.Add(d)
	c.mu.Unlock()
}

type IDs struct {
	mu   sync.Mutex
	next int64
}

func (g *IDs) NewID() domain.ID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.next++
	return domain.ID(1000 + g.next)
}

// Tokens returns token-1, token-2, …
type Tokens struct {
	mu sync.Mutex
	n  int
}

func (t *Tokens) NewToken() (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.n++
	return fmt.Sprintf("token-%d", t.n), nil
}

// Hasher stores "hashed:<password>".
type Hasher struct{}

func (Hasher) Hash(password string) (string, error) { return "hashed:" + password, nil }

func (Hasher) Verify(encoded, password string) (bool, error) {
	if !strings.HasPrefix(encoded, "hashed:") {
		return false, errors.New("malformed hash")
	}
	return encoded == "hashed:"+password, nil
}

// Limiter allows everything except keys starting with a prefix in Deny.
type Limiter struct {
	mu   sync.Mutex
	Deny map[string]time.Duration
	Keys []string
}

func (l *Limiter) Allow(key string, _ ports.Limit) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Keys = append(l.Keys, key)
	for prefix, d := range l.Deny {
		if strings.HasPrefix(key, prefix) {
			return false, d
		}
	}
	return true, 0
}

type Mailer struct {
	mu   sync.Mutex
	Off  bool
	Err  error
	Sent []ports.Mail
}

func (m *Mailer) Available(context.Context) bool { return !m.Off }

func (m *Mailer) Send(_ context.Context, msg ports.Mail) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	m.Sent = append(m.Sent, msg)
	return nil
}

// MailTester records messages sent with explicit settings.
type MailTester struct {
	Err      error
	Sent     []ports.Mail
	Settings []domain.SMTPSettings
}

func (m *MailTester) SendWith(_ context.Context, s domain.SMTPSettings, _ string, msg ports.Mail) error {
	if m.Err != nil {
		return m.Err
	}
	m.Settings = append(m.Settings, s)
	m.Sent = append(m.Sent, msg)
	return nil
}

type Secrets struct {
	mu     sync.Mutex
	Values map[string]string
}

func (s *Secrets) Secret(_ context.Context, key string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.Values[key]
	return v, ok, nil
}

func (s *Secrets) SetSecret(_ context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Values == nil {
		s.Values = map[string]string{}
	}
	if value == "" {
		delete(s.Values, key)
	} else {
		s.Values[key] = value
	}
	return nil
}

// Email is a message recorded by Emails.
type Email struct {
	Kind, To, Name, Locale, Link string
}

// Emails records the emails services ask to send.
type Emails struct {
	mu   sync.Mutex
	Err  error
	Sent []Email
}

func (e *Emails) VerifyEmail(_ context.Context, to, name, loc, link string) error {
	return e.record("verify", to, name, loc, link)
}

func (e *Emails) PasswordReset(_ context.Context, to, name, loc, link string) error {
	return e.record("reset", to, name, loc, link)
}

func (e *Emails) Test(_ context.Context, to, loc string) error {
	return e.record("test", to, "", loc, "")
}

func (e *Emails) record(kind, to, name, loc, link string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.Err != nil {
		return e.Err
	}
	e.Sent = append(e.Sent, Email{Kind: kind, To: to, Name: name, Locale: loc, Link: link})
	return nil
}

// Last returns the most recent email of kind.
func (e *Emails) Last(kind string) (Email, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, sent := range slices.Backward(e.Sent) {
		if sent.Kind == kind {
			return sent, true
		}
	}
	return Email{}, false
}
