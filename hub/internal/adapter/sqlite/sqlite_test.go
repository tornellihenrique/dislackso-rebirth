package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

var t0 = time.UnixMilli(1_790_000_000_000).UTC()

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(t.Context(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newUser(id domain.ID, username, email string) domain.User {
	return domain.User{
		ID:           id,
		Username:     username,
		Email:        email,
		PasswordHash: "hash",
		DisplayName:  username,
		StatusPref:   domain.StatusOnline,
		CreatedAt:    t0,
		UpdatedAt:    t0,
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	for range 2 {
		db, err := Open(t.Context(), path)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if err := db.Ping(t.Context()); err != nil {
			t.Fatalf("Ping: %v", err)
		}
		_ = db.Close()
	}
}

func TestUsers(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)

	alice := newUser(1, "Alice", "alice@example.com")
	if err := db.CreateUser(ctx, alice); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	conflicts := []struct {
		name  string
		user  domain.User
		field string
	}{
		{"username differs only in case", newUser(2, "ALICE", "other@example.com"), "username"},
		{"email differs only in case", newUser(3, "bob", "ALICE@example.com"), "email"},
	}
	for _, c := range conflicts {
		err := db.CreateUser(ctx, c.user)
		de, ok := domain.AsError(err)
		if !errors.Is(err, domain.ErrConflict) || !ok || len(de.Fields) != 1 || de.Fields[0].Field != c.field {
			t.Errorf("%s: err = %v, want conflict on %s", c.name, err, c.field)
		}
	}

	lookups := []struct {
		name string
		get  func() (domain.User, error)
	}{
		{"by id", func() (domain.User, error) { return db.UserByID(ctx, 1) }},
		{"by email, any case", func() (domain.User, error) { return db.UserByEmail(ctx, "Alice@Example.com") }},
		{"by login username", func() (domain.User, error) { return db.UserByLogin(ctx, "alice") }},
		{"by login email", func() (domain.User, error) { return db.UserByLogin(ctx, "alice@example.com") }},
	}
	for _, l := range lookups {
		got, err := l.get()
		if err != nil {
			t.Fatalf("%s: %v", l.name, err)
		}
		if d := cmp.Diff(alice, got); d != "" {
			t.Errorf("%s mismatch (-want +got):\n%s", l.name, d)
		}
	}

	if _, err := db.UserByLogin(ctx, "nobody"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown login: err = %v, want ErrNotFound", err)
	}

	verified, disabled := t0.Add(time.Hour), t0.Add(2*time.Hour)
	alice.DisplayName = "Alice A."
	alice.EmailVerifiedAt = &verified
	alice.DisabledAt = &disabled
	alice.IsInstanceAdmin = true
	alice.StatusPref = domain.StatusDND
	alice.CustomStatus = "focusing"
	alice.UpdatedAt = disabled
	if err := db.UpdateUser(ctx, alice); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	got, err := db.UserByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(alice, got); d != "" {
		t.Errorf("after update (-want +got):\n%s", d)
	}

	if err := db.UpdateUser(ctx, newUser(99, "ghost", "ghost@example.com")); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("update missing user: err = %v, want ErrNotFound", err)
	}

	if err := db.CreateUser(ctx, newUser(2, "bob", "bob@example.com")); err != nil {
		t.Fatal(err)
	}
	users, err := db.ListUsers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 || users[0].ID != 1 || users[1].ID != 2 {
		t.Errorf("ListUsers = %v, want ids 1, 2", users)
	}
	if n, err := db.CountUsers(ctx); err != nil || n != 2 {
		t.Errorf("CountUsers = %d, %v; want 2", n, err)
	}
}

func TestDeleteUser(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	if err := db.CreateUser(ctx, newUser(1, "alice", "alice@example.com")); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteUser(ctx, 1, t0.Add(time.Hour)); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	for name, lookup := range map[string]func() error{
		"by id":    func() error { _, err := db.UserByID(ctx, 1); return err },
		"by email": func() error { _, err := db.UserByEmail(ctx, "alice@example.com"); return err },
		"by login": func() error { _, err := db.UserByLogin(ctx, "alice"); return err },
	} {
		if err := lookup(); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("lookup %s err = %v, want not found", name, err)
		}
	}
	if n, _ := db.CountUsers(ctx); n != 0 {
		t.Errorf("CountUsers = %d, want 0", n)
	}
	// The username and email are free for a new account.
	if err := db.CreateUser(ctx, newUser(2, "alice", "alice@example.com")); err != nil {
		t.Errorf("reuse username and email: %v", err)
	}
	if err := db.DeleteUser(ctx, 1, t0); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("second delete err = %v, want not found", err)
	}
}

func TestSessions(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	for _, u := range []domain.User{newUser(1, "alice", "a@example.com"), newUser(2, "bob", "b@example.com")} {
		if err := db.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}

	mk := func(id, user domain.ID, hash string, expires time.Time) domain.Session {
		return domain.Session{
			ID: id, UserID: user, TokenHash: []byte(hash),
			CreatedAt: t0, LastSeenAt: t0, ExpiresAt: expires, UserAgent: "ua", IP: "127.0.0.1",
		}
	}
	sessions := []domain.Session{
		mk(10, 1, "h10", t0.Add(time.Hour)),
		mk(11, 1, "h11", t0.Add(time.Hour)),
		mk(12, 1, "h12", t0.Add(-time.Minute)),
		mk(13, 2, "h13", t0.Add(time.Hour)),
	}
	for _, s := range sessions {
		if err := db.CreateSession(ctx, s); err != nil {
			t.Fatal(err)
		}
	}

	got, err := db.SessionByTokenHash(ctx, []byte("h10"))
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(sessions[0], got); d != "" {
		t.Errorf("SessionByTokenHash (-want +got):\n%s", d)
	}

	later := t0.Add(5 * time.Minute)
	if err := db.TouchSession(ctx, 10, later, later.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.SessionByTokenHash(ctx, []byte("h10")); !got.LastSeenAt.Equal(later) {
		t.Errorf("LastSeenAt = %v, want %v", got.LastSeenAt, later)
	}

	active, err := db.ActiveSessions(ctx, 1, t0)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 || active[0].ID != 10 {
		t.Errorf("ActiveSessions = %v, want 10 (most recent) and 11", active)
	}

	if err := db.RevokeSession(ctx, 2, 10, t0); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("revoking another user's session: err = %v, want ErrNotFound", err)
	}
	if err := db.RevokeSession(ctx, 1, 11, t0); err != nil {
		t.Fatal(err)
	}
	if err := db.RevokeSession(ctx, 1, 11, t0); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("revoking twice: err = %v, want ErrNotFound", err)
	}

	n, err := db.RevokeUserSessions(ctx, 1, 10, t0)
	if err != nil || n != 1 {
		t.Errorf("RevokeUserSessions keeping 10 = %d, %v; want 1 (the expired one)", n, err)
	}
	if active, _ := db.ActiveSessions(ctx, 1, t0); len(active) != 1 || active[0].ID != 10 {
		t.Errorf("after revoking others: %v, want only 10", active)
	}
	if n, _ := db.RevokeUserSessions(ctx, 1, 0, t0); n != 1 {
		t.Errorf("RevokeUserSessions keeping none = %d, want 1", n)
	}
	if active, _ := db.ActiveSessions(ctx, 2, t0); len(active) != 1 {
		t.Errorf("other user's sessions changed: %v", active)
	}
}

func TestEmailTokens(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	if err := db.CreateUser(ctx, newUser(1, "alice", "a@example.com")); err != nil {
		t.Fatal(err)
	}
	tok := domain.EmailToken{
		ID: 20, UserID: 1, Purpose: domain.TokenPasswordReset, TokenHash: []byte("t20"),
		ExpiresAt: t0.Add(time.Hour), CreatedAt: t0,
	}
	invite := domain.EmailToken{
		ID: 21, UserID: 1, Purpose: domain.TokenInstanceInvite, TokenHash: []byte("t21"),
		Payload: "{}", ExpiresAt: t0.Add(time.Hour), CreatedAt: t0,
	}
	for _, x := range []domain.EmailToken{tok, invite} {
		if err := db.CreateEmailToken(ctx, x); err != nil {
			t.Fatal(err)
		}
	}

	got, err := db.EmailTokenByHash(ctx, []byte("t20"))
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(tok, got); d != "" {
		t.Errorf("EmailTokenByHash (-want +got):\n%s", d)
	}

	if err := db.UseEmailToken(ctx, 20, t0); err != nil {
		t.Fatal(err)
	}
	if err := db.UseEmailToken(ctx, 20, t0); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("second use: err = %v, want ErrNotFound", err)
	}

	invites, err := db.EmailTokensByPurpose(ctx, domain.TokenInstanceInvite)
	if err != nil || len(invites) != 1 || invites[0].ID != 21 {
		t.Errorf("EmailTokensByPurpose = %v, %v; want the invite", invites, err)
	}

	if err := db.ExpireUserTokens(ctx, 1, domain.TokenInstanceInvite, t0); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.EmailTokenByHash(ctx, []byte("t21")); got.UsedAt == nil {
		t.Error("ExpireUserTokens left the invite usable")
	}

	if err := db.DeleteEmailToken(ctx, 21, domain.TokenPasswordReset); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("delete with the wrong purpose: err = %v, want ErrNotFound", err)
	}
	if err := db.DeleteEmailToken(ctx, 21, domain.TokenInstanceInvite); err != nil {
		t.Fatal(err)
	}
}

func TestSettings(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	defaults := domain.DefaultInstanceSettings()

	got, err := db.LoadSettings(ctx, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(defaults, got); d != "" {
		t.Errorf("empty database should give the defaults (-want +got):\n%s", d)
	}

	completed := t0
	s := defaults
	s.Name = "Team"
	s.PublicURL = "https://hub.example.com"
	s.SetupCompletedAt = &completed
	s.RegistrationMode = domain.RegistrationOpen
	s.MembersCanInvite = false
	s.SMTP = domain.SMTPSettings{Enabled: true, Host: "smtp.example.com", Port: 465, Security: domain.SMTPTLS, FromAddress: "hub@example.com"}
	s.Limits.MaxUploadMB = 25
	if err := db.SaveSettings(ctx, s); err != nil {
		t.Fatal(err)
	}
	got, err = db.LoadSettings(ctx, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(s, got); d != "" {
		t.Errorf("round trip (-want +got):\n%s", d)
	}
}

func TestInTxRollsBack(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	boom := errors.New("boom")

	err := db.InTx(ctx, func(ctx context.Context) error {
		if err := db.CreateUser(ctx, newUser(1, "alice", "a@example.com")); err != nil {
			return err
		}
		if _, err := db.UserByID(ctx, 1); err != nil {
			t.Errorf("a transaction should read its own writes: %v", err)
		}
		return db.InTx(ctx, func(context.Context) error { return boom })
	})
	if !errors.Is(err, boom) {
		t.Fatalf("InTx err = %v, want boom", err)
	}
	if _, err := db.UserByID(ctx, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("user survived the rollback: %v", err)
	}
}
