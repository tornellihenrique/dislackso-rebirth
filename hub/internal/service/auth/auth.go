// Package auth registers users, signs them in and out, and handles email
// verification, email changes and password resets.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/internal/emailtoken"
)

const (
	// SessionLifetime is how long a session lives without use; every use
	// extends it (sliding expiry).
	SessionLifetime     = 30 * 24 * time.Hour
	sessionRefreshEvery = 10 * time.Minute

	verifyTokenTTL = 24 * time.Hour
	resetTokenTTL  = 30 * time.Minute
	// Admin-issued reset links are handed over by hand, so they get longer.
	adminResetLinkTTL = 24 * time.Hour

	userAgentMaxLen = 256
)

var (
	limitLoginIP      = ports.Limit{Burst: 10, Every: 6 * time.Second}
	limitLoginAccount = ports.Limit{Burst: 5, Every: time.Minute}
	limitRegisterIP   = ports.Limit{Burst: 5, Every: 10 * time.Minute}
	limitTokenIP      = ports.Limit{Burst: 10, Every: 30 * time.Second}
	limitResendUser   = ports.Limit{Burst: 2, Every: 2 * time.Minute}
	limitResetIP      = ports.Limit{Burst: 5, Every: 3 * time.Minute}
	limitResetEmail   = ports.Limit{Burst: 3, Every: 10 * time.Minute}
)

type SettingsReader interface {
	Get(ctx context.Context) (domain.InstanceSettings, error)
}

// MailAvailability tells whether the instance can send email; when it can,
// new accounts must verify their address.
type MailAvailability interface {
	Available(ctx context.Context) bool
}

type Emails interface {
	VerifyEmail(ctx context.Context, to, name, locale, link string) error
	PasswordReset(ctx context.Context, to, name, locale, link string) error
	ConfirmEmailChange(ctx context.Context, to, name, locale, link string) error
}

// InviteClaimer redeems registration invites inside the registration
// transaction. The returned function runs once the user exists, e.g. to add
// them to the inviting guild.
type InviteClaimer interface {
	ClaimForRegistration(ctx context.Context, code string) (func(ctx context.Context, userID domain.ID) error, error)
}

type Deps struct {
	Users    ports.UserRepo
	Sessions ports.SessionRepo
	Tokens   ports.EmailTokenRepo
	Tx       ports.TxRunner
	Hasher   ports.PasswordHasher
	TokenGen ports.TokenGen
	IDs      ports.IDGen
	Clock    ports.Clock
	Limiter  ports.RateLimiter
	Mail     MailAvailability
	Settings SettingsReader
	Emails   Emails
	Invites  InviteClaimer
	Events   ports.EventPublisher
	Logger   *slog.Logger
}

type Service struct {
	d         Deps
	tokens    emailtoken.Store
	dummyHash func() string
}

func New(d Deps) *Service {
	s := &Service{
		d:      d,
		tokens: emailtoken.Store{Repo: d.Tokens, Tokens: d.TokenGen, IDs: d.IDs, Clock: d.Clock},
	}
	s.dummyHash = sync.OnceValue(func() string {
		h, err := d.Hasher.Hash("placeholder password for unknown users")
		if err != nil {
			return ""
		}
		return h
	})
	return s
}

// SignedIn is a new session; Token goes to the client once and is never stored.
type SignedIn struct {
	User    domain.User
	Session domain.Session
	Token   string
}

// Principal is an authenticated request.
type Principal struct {
	Actor   domain.Actor
	User    domain.User
	Session domain.Session
	// Refreshed reports that the session's expiry moved, so the cookie
	// should be renewed.
	Refreshed bool
}

type SetupState struct {
	Complete           bool
	InstanceName       string
	RegistrationMode   domain.RegistrationMode
	EmailVerification  bool
	SetupAvailableHere bool
}

func (s *Service) SetupState(ctx context.Context, client domain.ClientInfo) (SetupState, error) {
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return SetupState{}, err
	}
	return SetupState{
		Complete:           st.SetupComplete(),
		InstanceName:       st.Name,
		RegistrationMode:   st.RegistrationMode,
		EmailVerification:  s.VerificationRequired(ctx),
		SetupAvailableHere: !st.SetupComplete() && client.IsLocal(),
	}, nil
}

// VerificationRequired reports whether accounts must confirm their email.
func (s *Service) VerificationRequired(ctx context.Context) bool {
	return s.d.Mail.Available(ctx)
}

type RegisterInput struct {
	Username   string
	Email      string
	Password   string
	InviteCode string
}

func (s *Service) Register(ctx context.Context, client domain.ClientInfo, in RegisterInput) (SignedIn, error) {
	if err := s.allow("register:ip:"+client.IP.String(), limitRegisterIP); err != nil {
		return SignedIn{}, err
	}
	st, err := s.ready(ctx)
	if err != nil {
		return SignedIn{}, err
	}
	if st.RegistrationMode == domain.RegistrationClosed {
		return SignedIn{}, domain.Forbidden(domain.ReasonRegistrationClosed)
	}

	username := strings.TrimSpace(in.Username)
	email := domain.NormalizeEmail(in.Email)
	code := strings.TrimSpace(in.InviteCode)
	checks := []*domain.FieldViolation{
		domain.ValidateUsername("username", username),
		domain.ValidateEmail("email", email),
		domain.ValidatePassword("password", in.Password),
	}
	if st.RegistrationMode == domain.RegistrationInviteOnly && code == "" {
		checks = append(checks, &domain.FieldViolation{Field: "invite_code", Reason: domain.ReasonFieldRequired})
	}
	if err := domain.Violations(checks...); err != nil {
		return SignedIn{}, err
	}

	hash, err := s.d.Hasher.Hash(in.Password)
	if err != nil {
		return SignedIn{}, fmt.Errorf("hash password: %w", err)
	}
	verify := s.VerificationRequired(ctx)
	now := s.d.Clock.Now()
	user := domain.User{
		ID:           s.d.IDs.NewID(),
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		DisplayName:  username,
		StatusPref:   domain.StatusOnline,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	var signed SignedIn
	var verifyToken string
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		var afterCreate func(context.Context, domain.ID) error
		if code != "" {
			var err error
			if afterCreate, err = s.d.Invites.ClaimForRegistration(ctx, code); err != nil {
				return err
			}
		}
		if err := s.d.Users.CreateUser(ctx, user); err != nil {
			return err
		}
		if afterCreate != nil {
			if err := afterCreate(ctx, user.ID); err != nil {
				return err
			}
		}
		if verify {
			var err error
			if verifyToken, _, err = s.tokens.Issue(ctx, domain.TokenVerifyEmail, user.ID, verifyTokenTTL, user.Email); err != nil {
				return err
			}
		}
		var err error
		signed, err = s.startSession(ctx, client, user)
		return err
	})
	if err != nil {
		return SignedIn{}, err
	}
	if verify {
		// The account exists either way; the user can ask for another email.
		if err := s.sendVerification(ctx, client.Locale, user, verifyToken); err != nil {
			s.d.Logger.WarnContext(ctx, "send verification email", slog.String("user_id", user.ID.String()), slog.Any("error", err))
		}
	}
	return signed, nil
}

func (s *Service) Login(ctx context.Context, client domain.ClientInfo, login, password string) (SignedIn, error) {
	login = strings.TrimSpace(login)
	if err := s.allow("login:ip:"+client.IP.String(), limitLoginIP); err != nil {
		return SignedIn{}, err
	}
	if err := s.allow("login:account:"+strings.ToLower(login), limitLoginAccount); err != nil {
		return SignedIn{}, err
	}
	if _, err := s.ready(ctx); err != nil {
		return SignedIn{}, err
	}
	if login == "" || password == "" {
		return SignedIn{}, domain.Unauthenticated(domain.ReasonInvalidCredentials)
	}

	user, err := s.d.Users.UserByLogin(ctx, login)
	if errors.Is(err, domain.ErrNotFound) {
		// Spend the same time as a real check, so response time does not
		// reveal which accounts exist.
		_, _ = s.d.Hasher.Verify(s.dummyHash(), password)
		return SignedIn{}, domain.Unauthenticated(domain.ReasonInvalidCredentials)
	}
	if err != nil {
		return SignedIn{}, err
	}
	ok, err := s.d.Hasher.Verify(user.PasswordHash, password)
	if err != nil {
		return SignedIn{}, fmt.Errorf("verify password of user %d: %w", user.ID, err)
	}
	if !ok {
		return SignedIn{}, domain.Unauthenticated(domain.ReasonInvalidCredentials)
	}
	if user.Disabled() {
		return SignedIn{}, domain.Forbidden(domain.ReasonAccountDisabled)
	}
	return s.startSession(ctx, client, user)
}

// StartSession signs in a user the caller already authenticated, such as the
// admin created by the setup wizard.
func (s *Service) StartSession(ctx context.Context, client domain.ClientInfo, user domain.User) (SignedIn, error) {
	return s.startSession(ctx, client, user)
}

func (s *Service) startSession(ctx context.Context, client domain.ClientInfo, user domain.User) (SignedIn, error) {
	raw, err := s.d.TokenGen.NewToken()
	if err != nil {
		return SignedIn{}, err
	}
	now := s.d.Clock.Now()
	sess := domain.Session{
		ID:         s.d.IDs.NewID(),
		UserID:     user.ID,
		TokenHash:  domain.HashToken(raw),
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(SessionLifetime),
		UserAgent:  truncate(client.UserAgent, userAgentMaxLen),
		IP:         ipString(client),
	}
	if err := s.d.Sessions.CreateSession(ctx, sess); err != nil {
		return SignedIn{}, err
	}
	return SignedIn{User: user, Session: sess, Token: raw}, nil
}

// Authenticate resolves a session token, extending the session when it has
// not been used for a while.
func (s *Service) Authenticate(ctx context.Context, raw string) (Principal, error) {
	expired := domain.Unauthenticated(domain.ReasonSessionExpired)
	if raw == "" {
		return Principal{}, expired
	}
	sess, err := s.d.Sessions.SessionByTokenHash(ctx, domain.HashToken(raw))
	if errors.Is(err, domain.ErrNotFound) {
		return Principal{}, expired
	}
	if err != nil {
		return Principal{}, err
	}
	now := s.d.Clock.Now()
	if !sess.Active(now) {
		return Principal{}, expired
	}
	user, err := s.d.Users.UserByID(ctx, sess.UserID)
	if errors.Is(err, domain.ErrNotFound) {
		return Principal{}, expired
	}
	if err != nil {
		return Principal{}, err
	}
	if user.Disabled() {
		return Principal{}, domain.Unauthenticated(domain.ReasonAccountDisabled)
	}

	refreshed := false
	if now.Sub(sess.LastSeenAt) >= sessionRefreshEvery {
		sess.LastSeenAt, sess.ExpiresAt = now, now.Add(SessionLifetime)
		if err := s.d.Sessions.TouchSession(ctx, sess.ID, sess.LastSeenAt, sess.ExpiresAt); err != nil {
			return Principal{}, err
		}
		refreshed = true
	}
	return Principal{Actor: ActorFor(user, sess), User: user, Session: sess, Refreshed: refreshed}, nil
}

func ActorFor(u domain.User, s domain.Session) domain.Actor {
	return domain.Actor{
		UserID:        u.ID,
		SessionID:     s.ID,
		InstanceAdmin: u.IsInstanceAdmin,
		EmailVerified: u.EmailVerified(),
	}
}

func (s *Service) Logout(ctx context.Context, actor domain.Actor) error {
	err := s.d.Sessions.RevokeSession(ctx, actor.UserID, actor.SessionID, s.d.Clock.Now())
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	return err
}

func (s *Service) VerifyEmail(ctx context.Context, client domain.ClientInfo, raw string) error {
	if err := s.allow("token:ip:"+client.IP.String(), limitTokenIP); err != nil {
		return err
	}
	return s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		tok, err := s.tokens.Redeem(ctx, domain.TokenVerifyEmail, raw)
		if err != nil {
			return err
		}
		user, err := s.d.Users.UserByID(ctx, tok.UserID)
		if err != nil {
			return err
		}
		// The token confirms the address it was sent to, not a newer one.
		if !strings.EqualFold(user.Email, tok.Payload) {
			return domain.NotFound(domain.ReasonTokenInvalid)
		}
		if user.EmailVerified() {
			return nil
		}
		now := s.d.Clock.Now()
		user.EmailVerifiedAt, user.UpdatedAt = &now, now
		return s.d.Users.UpdateUser(ctx, user)
	})
}

func (s *Service) ResendVerification(ctx context.Context, client domain.ClientInfo, actor domain.Actor) error {
	if !s.VerificationRequired(ctx) {
		return domain.Precondition(domain.ReasonEmailUnavailable)
	}
	user, err := s.d.Users.UserByID(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if user.EmailVerified() {
		return domain.Precondition(domain.ReasonEmailAlreadyVerified)
	}
	if err := s.allow("verify:user:"+user.ID.String(), limitResendUser); err != nil {
		return err
	}
	var raw string
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Tokens.ExpireUserTokens(ctx, user.ID, domain.TokenVerifyEmail, s.d.Clock.Now()); err != nil {
			return err
		}
		var err error
		raw, _, err = s.tokens.Issue(ctx, domain.TokenVerifyEmail, user.ID, verifyTokenTTL, user.Email)
		return err
	})
	if err != nil {
		return err
	}
	if err := s.sendVerification(ctx, client.Locale, user, raw); err != nil {
		s.d.Logger.WarnContext(ctx, "send verification email", slog.String("user_id", user.ID.String()), slog.Any("error", err))
		return &domain.Error{Kind: domain.ErrFailedPrecondition, Reason: domain.ReasonEmailDeliveryFailed}
	}
	return nil
}

// RequestPasswordReset emails a reset link. It answers the same whether or
// not the account exists.
func (s *Service) RequestPasswordReset(ctx context.Context, client domain.ClientInfo, email string) error {
	email = domain.NormalizeEmail(email)
	if v := domain.ValidateEmail("email", email); v != nil {
		return domain.Invalid(*v)
	}
	if err := s.allow("reset:ip:"+client.IP.String(), limitResetIP); err != nil {
		return err
	}
	if err := s.allow("reset:email:"+strings.ToLower(email), limitResetEmail); err != nil {
		return err
	}
	if _, err := s.ready(ctx); err != nil {
		return err
	}
	if !s.VerificationRequired(ctx) {
		return domain.Precondition(domain.ReasonEmailUnavailable)
	}

	user, err := s.d.Users.UserByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if user.Disabled() {
		return nil
	}
	raw, _, err := s.issueReset(ctx, user.ID, resetTokenTTL)
	if err != nil {
		return err
	}
	link, err := s.link(ctx, "/reset-password", raw)
	if err != nil {
		return err
	}
	if err := s.d.Emails.PasswordReset(ctx, user.Email, user.DisplayName, client.Locale, link); err != nil {
		// Reporting the failure would reveal that the account exists.
		s.d.Logger.WarnContext(ctx, "send password reset email", slog.String("user_id", user.ID.String()), slog.Any("error", err))
	}
	return nil
}

// ResetPassword sets a new password and signs out every session.
func (s *Service) ResetPassword(ctx context.Context, client domain.ClientInfo, raw, newPassword string) error {
	if err := s.allow("token:ip:"+client.IP.String(), limitTokenIP); err != nil {
		return err
	}
	if v := domain.ValidatePassword("new_password", newPassword); v != nil {
		return domain.Invalid(*v)
	}
	hash, err := s.d.Hasher.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		tok, err := s.tokens.Redeem(ctx, domain.TokenPasswordReset, raw)
		if err != nil {
			return err
		}
		user, err := s.d.Users.UserByID(ctx, tok.UserID)
		if err != nil {
			return err
		}
		now := s.d.Clock.Now()
		user.PasswordHash, user.UpdatedAt = hash, now
		if err := s.d.Users.UpdateUser(ctx, user); err != nil {
			return err
		}
		if err := s.d.Tokens.ExpireUserTokens(ctx, user.ID, domain.TokenPasswordReset, now); err != nil {
			return err
		}
		_, err = s.d.Sessions.RevokeUserSessions(ctx, user.ID, 0, now)
		return err
	})
}

// IssuePasswordResetLink creates a reset link for an admin to hand over,
// for instances without email. Authorization is the caller's job.
func (s *Service) IssuePasswordResetLink(ctx context.Context, userID domain.ID) (string, time.Time, error) {
	if _, err := s.d.Users.UserByID(ctx, userID); errors.Is(err, domain.ErrNotFound) {
		return "", time.Time{}, domain.NotFound(domain.ReasonUserNotFound)
	} else if err != nil {
		return "", time.Time{}, err
	}
	raw, tok, err := s.issueReset(ctx, userID, adminResetLinkTTL)
	if err != nil {
		return "", time.Time{}, err
	}
	link, err := s.link(ctx, "/reset-password", raw)
	if err != nil {
		return "", time.Time{}, err
	}
	return link, tok.ExpiresAt, nil
}

func (s *Service) issueReset(ctx context.Context, userID domain.ID, ttl time.Duration) (string, domain.EmailToken, error) {
	var raw string
	var tok domain.EmailToken
	err := s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Tokens.ExpireUserTokens(ctx, userID, domain.TokenPasswordReset, s.d.Clock.Now()); err != nil {
			return err
		}
		var err error
		raw, tok, err = s.tokens.Issue(ctx, domain.TokenPasswordReset, userID, ttl, "")
		return err
	})
	return raw, tok, err
}

func (s *Service) sendVerification(ctx context.Context, loc string, user domain.User, raw string) error {
	link, err := s.link(ctx, "/verify-email", raw)
	if err != nil {
		return err
	}
	return s.d.Emails.VerifyEmail(ctx, user.Email, user.DisplayName, loc, link)
}

// link builds an absolute link from the configured public URL. Request
// headers are never used: a forged Host would leak the token elsewhere.
func (s *Service) link(ctx context.Context, path, token string) (string, error) {
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return "", err
	}
	return st.PublicURL + path + "?token=" + url.QueryEscape(token), nil
}

func (s *Service) ready(ctx context.Context) (domain.InstanceSettings, error) {
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return domain.InstanceSettings{}, err
	}
	if !st.SetupComplete() {
		return domain.InstanceSettings{}, domain.Precondition(domain.ReasonSetupRequired)
	}
	return st, nil
}

func (s *Service) allow(key string, limit ports.Limit) error {
	if ok, retry := s.d.Limiter.Allow(key, limit); !ok {
		return domain.RateLimited(retry)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func ipString(c domain.ClientInfo) string {
	if !c.IP.IsValid() {
		return ""
	}
	return c.IP.Unmap().String()
}
