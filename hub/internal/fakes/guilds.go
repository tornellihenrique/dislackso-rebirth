package fakes

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

type memberKey struct{ guild, user domain.ID }

type guildData struct {
	guilds   map[domain.ID]domain.Guild
	members  map[memberKey]domain.Member
	channels map[domain.ID]domain.Channel
	invites  map[string]domain.Invite
	bans     map[memberKey]domain.Ban
	audit    []domain.AuditEntry
	files    map[domain.ID]domain.File
}

func newGuildData() guildData {
	return guildData{
		guilds:   map[domain.ID]domain.Guild{},
		members:  map[memberKey]domain.Member{},
		channels: map[domain.ID]domain.Channel{},
		invites:  map[string]domain.Invite{},
		bans:     map[memberKey]domain.Ban{},
		files:    map[domain.ID]domain.File{},
	}
}

func (s *Store) cloneGuildData() guildData {
	return guildData{
		guilds:   maps.Clone(s.guilds),
		members:  maps.Clone(s.members),
		channels: maps.Clone(s.channels),
		invites:  maps.Clone(s.invites),
		bans:     maps.Clone(s.bans),
		audit:    slices.Clone(s.audit),
		files:    maps.Clone(s.files),
	}
}

func (s *Store) GuildUsers(_ context.Context, guildID domain.ID) ([]domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.User
	for k := range s.members {
		if k.guild == guildID {
			if u, ok := s.users[k.user]; ok {
				out = append(out, u)
			}
		}
	}
	slices.SortFunc(out, func(a, b domain.User) int { return cmp.Compare(a.ID, b.ID) })
	return out, nil
}

func (s *Store) CreateGuild(_ context.Context, g domain.Guild) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.guilds[g.ID] = g
	return nil
}

func (s *Store) GuildByID(_ context.Context, id domain.ID) (domain.Guild, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.guilds[id]
	if !ok {
		return domain.Guild{}, domain.ErrNotFound
	}
	return g, nil
}

func (s *Store) UpdateGuild(_ context.Context, g domain.Guild) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.guilds[g.ID]; !ok {
		return domain.ErrNotFound
	}
	s.guilds[g.ID] = g
	return nil
}

func (s *Store) DeleteGuild(_ context.Context, id domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.guilds[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.guilds, id)
	maps.DeleteFunc(s.members, func(k memberKey, _ domain.Member) bool { return k.guild == id })
	maps.DeleteFunc(s.bans, func(k memberKey, _ domain.Ban) bool { return k.guild == id })
	maps.DeleteFunc(s.channels, func(_ domain.ID, c domain.Channel) bool { return c.GuildID == id })
	maps.DeleteFunc(s.invites, func(_ string, i domain.Invite) bool { return i.GuildID == id })
	s.audit = slices.DeleteFunc(s.audit, func(e domain.AuditEntry) bool { return e.GuildID == id })
	return nil
}

func (s *Store) CountGuildsOwnedBy(_ context.Context, userID domain.ID) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, g := range s.guilds {
		if g.OwnerID == userID {
			n++
		}
	}
	return n, nil
}

func (s *Store) UserGuilds(ctx context.Context, userID domain.ID) ([]domain.Guild, error) {
	ms, _ := s.UserMemberships(ctx, userID)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Guild, 0, len(ms))
	for _, m := range ms {
		out = append(out, s.guilds[m.GuildID])
	}
	return out, nil
}

func (s *Store) AddMember(_ context.Context, m domain.Member) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := memberKey{m.GuildID, m.UserID}
	if _, ok := s.members[k]; ok {
		return domain.Conflict("guild_id")
	}
	s.members[k] = m
	return nil
}

func (s *Store) MemberByID(_ context.Context, guildID, userID domain.ID) (domain.Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.members[memberKey{guildID, userID}]
	if !ok {
		return domain.Member{}, domain.ErrNotFound
	}
	return m, nil
}

func (s *Store) UpdateMemberRole(_ context.Context, guildID, userID domain.ID, role domain.GuildRole, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := memberKey{guildID, userID}
	m, ok := s.members[k]
	if !ok {
		return domain.ErrNotFound
	}
	m.Role, m.UpdatedAt = role, at
	s.members[k] = m
	return nil
}

func (s *Store) RemoveMember(_ context.Context, guildID, userID domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := memberKey{guildID, userID}
	if _, ok := s.members[k]; !ok {
		return domain.ErrNotFound
	}
	delete(s.members, k)
	return nil
}

func (s *Store) GuildMembers(_ context.Context, guildID domain.ID) ([]domain.Member, error) {
	return s.filterMembers(func(m domain.Member) bool { return m.GuildID == guildID }), nil
}

func (s *Store) UserMemberships(_ context.Context, userID domain.ID) ([]domain.Member, error) {
	return s.filterMembers(func(m domain.Member) bool { return m.UserID == userID }), nil
}

func (s *Store) filterMembers(keep func(domain.Member) bool) []domain.Member {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Member
	for _, m := range s.members {
		if keep(m) {
			out = append(out, m)
		}
	}
	slices.SortFunc(out, func(a, b domain.Member) int {
		return cmp.Or(a.JoinedAt.Compare(b.JoinedAt), cmp.Compare(a.UserID, b.UserID), cmp.Compare(a.GuildID, b.GuildID))
	})
	return out
}

func (s *Store) CountMembers(ctx context.Context, guildID domain.ID) (int, error) {
	ms, _ := s.GuildMembers(ctx, guildID)
	return len(ms), nil
}

func (s *Store) CreateChannel(_ context.Context, c domain.Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channels[c.ID] = c
	return nil
}

func (s *Store) ChannelByID(_ context.Context, id domain.ID) (domain.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.channels[id]
	if !ok {
		return domain.Channel{}, domain.ErrNotFound
	}
	return c, nil
}

func (s *Store) UpdateChannel(_ context.Context, c domain.Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.channels[c.ID]; !ok {
		return domain.ErrNotFound
	}
	s.channels[c.ID] = c
	return nil
}

func (s *Store) DeleteChannel(_ context.Context, id domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.channels[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.channels, id)
	for cid, c := range s.channels {
		if c.ParentID == id {
			c.ParentID = 0
			s.channels[cid] = c
		}
	}
	return nil
}

func (s *Store) GuildChannels(_ context.Context, guildID domain.ID) ([]domain.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Channel
	for _, c := range s.channels {
		if c.GuildID == guildID {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b domain.Channel) int {
		return cmp.Or(cmp.Compare(a.Position, b.Position), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

func (s *Store) CountChannels(ctx context.Context, guildID domain.ID) (int, error) {
	cs, _ := s.GuildChannels(ctx, guildID)
	return len(cs), nil
}

func (s *Store) CreateInvite(_ context.Context, inv domain.Invite) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.invites[inv.Code]; ok {
		return domain.Conflict("code")
	}
	s.invites[inv.Code] = inv
	return nil
}

func (s *Store) InviteByCode(_ context.Context, code string) (domain.Invite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.invites[code]
	if !ok {
		return domain.Invite{}, domain.ErrNotFound
	}
	return inv, nil
}

func (s *Store) UseInvite(_ context.Context, code string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.invites[code]
	if !ok || !inv.Usable(now) {
		return domain.ErrNotFound
	}
	inv.Uses++
	s.invites[code] = inv
	return nil
}

func (s *Store) RevokeInvite(_ context.Context, code string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.invites[code]
	if !ok || inv.RevokedAt != nil {
		return domain.ErrNotFound
	}
	inv.RevokedAt = &at
	s.invites[code] = inv
	return nil
}

func (s *Store) UsableInvites(_ context.Context, guildID domain.ID, now time.Time) ([]domain.Invite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Invite
	for _, inv := range s.invites {
		if inv.GuildID == guildID && inv.Usable(now) {
			out = append(out, inv)
		}
	}
	slices.SortFunc(out, func(a, b domain.Invite) int {
		return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), cmp.Compare(a.Code, b.Code))
	})
	return out, nil
}

func (s *Store) CreateBan(_ context.Context, b domain.Ban) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := memberKey{b.GuildID, b.UserID}
	if _, ok := s.bans[k]; ok {
		return domain.Conflict("guild_id")
	}
	s.bans[k] = b
	return nil
}

func (s *Store) BanByID(_ context.Context, guildID, userID domain.ID) (domain.Ban, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bans[memberKey{guildID, userID}]
	if !ok {
		return domain.Ban{}, domain.ErrNotFound
	}
	return b, nil
}

func (s *Store) DeleteBan(_ context.Context, guildID, userID domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := memberKey{guildID, userID}
	if _, ok := s.bans[k]; !ok {
		return domain.ErrNotFound
	}
	delete(s.bans, k)
	return nil
}

func (s *Store) GuildBans(_ context.Context, guildID domain.ID) ([]domain.Ban, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Ban
	for _, b := range s.bans {
		if b.GuildID == guildID {
			out = append(out, b)
		}
	}
	slices.SortFunc(out, func(a, b domain.Ban) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return out, nil
}

func (s *Store) AppendAudit(_ context.Context, e domain.AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audit = append(s.audit, e)
	return nil
}

func (s *Store) AuditLog(_ context.Context, guildID, before domain.ID, limit int) ([]domain.AuditEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.AuditEntry
	for _, e := range slices.Backward(s.audit) {
		if e.GuildID == guildID && (before == 0 || e.ID < before) && len(out) < limit {
			out = append(out, e)
		}
	}
	return out, nil
}

// Audit returns every audit entry of a guild, oldest first.
func (s *Store) Audit(guildID domain.ID) []domain.AuditEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.AuditEntry
	for _, e := range s.audit {
		if e.GuildID == guildID {
			out = append(out, e)
		}
	}
	return out
}

func (s *Store) CreateFile(_ context.Context, f domain.File) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[f.ID] = f
	return nil
}

func (s *Store) FileByID(_ context.Context, id domain.ID) (domain.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.files[id]
	if !ok {
		return domain.File{}, domain.ErrNotFound
	}
	return f, nil
}
