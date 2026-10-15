package fakes

import (
	"bytes"
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

// Store implements the repositories and TxRunner in memory. InTx restores
// the previous state when fn fails, like a rolled-back transaction.
type Store struct {
	mu       sync.Mutex
	users    map[domain.ID]domain.User
	sessions map[domain.ID]domain.Session
	tokens   map[domain.ID]domain.EmailToken
	settings *domain.InstanceSettings
	inTx     bool
}

func NewStore() *Store {
	return &Store{
		users:    map[domain.ID]domain.User{},
		sessions: map[domain.ID]domain.Session{},
		tokens:   map[domain.ID]domain.EmailToken{},
	}
}

func (s *Store) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	s.mu.Lock()
	if s.inTx {
		s.mu.Unlock()
		return fn(ctx)
	}
	s.inTx = true
	users, sessions, tokens, settings := maps.Clone(s.users), maps.Clone(s.sessions), maps.Clone(s.tokens), s.settings
	s.mu.Unlock()

	err := fn(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.inTx = false
	if err != nil {
		s.users, s.sessions, s.tokens, s.settings = users, sessions, tokens, settings
	}
	return err
}

func (s *Store) CreateUser(_ context.Context, u domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkUnique(u); err != nil {
		return err
	}
	s.users[u.ID] = u
	return nil
}

func (s *Store) checkUnique(u domain.User) error {
	for _, other := range s.users {
		if other.ID == u.ID {
			continue
		}
		if strings.EqualFold(other.Username, u.Username) {
			return domain.Conflict("username")
		}
		if strings.EqualFold(other.Email, u.Email) {
			return domain.Conflict("email")
		}
	}
	return nil
}

func (s *Store) UserByID(_ context.Context, id domain.ID) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (s *Store) UserByEmail(_ context.Context, email string) (domain.User, error) {
	return s.findUser(func(u domain.User) bool { return strings.EqualFold(u.Email, email) })
}

func (s *Store) UserByLogin(_ context.Context, login string) (domain.User, error) {
	return s.findUser(func(u domain.User) bool {
		return strings.EqualFold(u.Username, login) || strings.EqualFold(u.Email, login)
	})
}

func (s *Store) findUser(match func(domain.User) bool) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if match(u) {
			return u, nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}

func (s *Store) UpdateUser(_ context.Context, u domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[u.ID]; !ok {
		return domain.ErrNotFound
	}
	if err := s.checkUnique(u); err != nil {
		return err
	}
	s.users[u.ID] = u
	return nil
}

func (s *Store) ListUsers(context.Context) ([]domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.SortedFunc(maps.Values(s.users), func(a, b domain.User) int { return int(a.ID - b.ID) }), nil
}

func (s *Store) CountUsers(context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.users), nil
}

func (s *Store) CreateSession(_ context.Context, sess domain.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = sess
	return nil
}

func (s *Store) SessionByTokenHash(_ context.Context, hash []byte) (domain.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sess := range s.sessions {
		if bytes.Equal(sess.TokenHash, hash) {
			return sess, nil
		}
	}
	return domain.Session{}, domain.ErrNotFound
}

func (s *Store) TouchSession(_ context.Context, id domain.ID, lastSeen, expires time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return domain.ErrNotFound
	}
	sess.LastSeenAt, sess.ExpiresAt = lastSeen, expires
	s.sessions[id] = sess
	return nil
}

func (s *Store) RevokeSession(_ context.Context, userID, id domain.ID, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok || sess.UserID != userID || sess.RevokedAt != nil {
		return domain.ErrNotFound
	}
	sess.RevokedAt = &at
	s.sessions[id] = sess
	return nil
}

func (s *Store) RevokeUserSessions(_ context.Context, userID, keep domain.ID, at time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for id, sess := range s.sessions {
		if sess.UserID == userID && id != keep && sess.RevokedAt == nil {
			sess.RevokedAt = &at
			s.sessions[id] = sess
			n++
		}
	}
	return n, nil
}

func (s *Store) ActiveSessions(_ context.Context, userID domain.ID, now time.Time) ([]domain.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Session
	for _, sess := range s.sessions {
		if sess.UserID == userID && sess.Active(now) {
			out = append(out, sess)
		}
	}
	slices.SortFunc(out, func(a, b domain.Session) int { return b.LastSeenAt.Compare(a.LastSeenAt) })
	return out, nil
}

func (s *Store) CreateEmailToken(_ context.Context, t domain.EmailToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[t.ID] = t
	return nil
}

func (s *Store) EmailTokenByHash(_ context.Context, hash []byte) (domain.EmailToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tokens {
		if bytes.Equal(t.TokenHash, hash) {
			return t, nil
		}
	}
	return domain.EmailToken{}, domain.ErrNotFound
}

func (s *Store) UseEmailToken(_ context.Context, id domain.ID, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tokens[id]
	if !ok || t.UsedAt != nil {
		return domain.ErrNotFound
	}
	t.UsedAt = &at
	s.tokens[id] = t
	return nil
}

func (s *Store) ExpireUserTokens(_ context.Context, userID domain.ID, purpose domain.TokenPurpose, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, t := range s.tokens {
		if t.UserID == userID && t.Purpose == purpose && t.UsedAt == nil {
			t.UsedAt = &at
			s.tokens[id] = t
		}
	}
	return nil
}

func (s *Store) EmailTokensByPurpose(_ context.Context, purpose domain.TokenPurpose) ([]domain.EmailToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.EmailToken
	for _, t := range s.tokens {
		if t.Purpose == purpose {
			out = append(out, t)
		}
	}
	slices.SortFunc(out, func(a, b domain.EmailToken) int { return int(b.ID - a.ID) })
	return out, nil
}

func (s *Store) DeleteEmailToken(_ context.Context, id domain.ID, purpose domain.TokenPurpose) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tokens[id]
	if !ok || t.Purpose != purpose {
		return domain.ErrNotFound
	}
	delete(s.tokens, id)
	return nil
}

func (s *Store) LoadSettings(_ context.Context, defaults domain.InstanceSettings) (domain.InstanceSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settings == nil {
		return defaults, nil
	}
	return *s.settings, nil
}

func (s *Store) SaveSettings(_ context.Context, st domain.InstanceSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = &st
	return nil
}
