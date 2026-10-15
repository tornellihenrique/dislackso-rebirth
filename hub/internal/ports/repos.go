package ports

import (
	"context"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

// Repositories return domain.ErrNotFound when a lookup finds nothing and
// domain.ErrConflict (with the field) when a unique column is taken.

type UserRepo interface {
	CreateUser(ctx context.Context, u domain.User) error
	UserByID(ctx context.Context, id domain.ID) (domain.User, error)
	UserByEmail(ctx context.Context, email string) (domain.User, error)
	// UserByLogin matches the username or the email, case-insensitively.
	UserByLogin(ctx context.Context, login string) (domain.User, error)
	// UpdateUser writes every mutable field of u.
	UpdateUser(ctx context.Context, u domain.User) error
	ListUsers(ctx context.Context) ([]domain.User, error)
	// GuildUsers returns the users who are members of the guild.
	GuildUsers(ctx context.Context, guildID domain.ID) ([]domain.User, error)
	CountUsers(ctx context.Context) (int, error)
}

type SessionRepo interface {
	CreateSession(ctx context.Context, s domain.Session) error
	SessionByTokenHash(ctx context.Context, hash []byte) (domain.Session, error)
	TouchSession(ctx context.Context, id domain.ID, lastSeen, expires time.Time) error
	// RevokeSession revokes one active session of the user.
	RevokeSession(ctx context.Context, userID, id domain.ID, at time.Time) error
	// RevokeUserSessions revokes every active session except keep (zero keeps none).
	RevokeUserSessions(ctx context.Context, userID, keep domain.ID, at time.Time) (int, error)
	ActiveSessions(ctx context.Context, userID domain.ID, now time.Time) ([]domain.Session, error)
}

type EmailTokenRepo interface {
	CreateEmailToken(ctx context.Context, t domain.EmailToken) error
	EmailTokenByHash(ctx context.Context, hash []byte) (domain.EmailToken, error)
	// UseEmailToken marks the token used; it fails with ErrNotFound when the
	// token was already used, so two concurrent uses cannot both succeed.
	UseEmailToken(ctx context.Context, id domain.ID, at time.Time) error
	// ExpireUserTokens marks every unused token of a purpose as used.
	ExpireUserTokens(ctx context.Context, userID domain.ID, purpose domain.TokenPurpose, at time.Time) error
	EmailTokensByPurpose(ctx context.Context, purpose domain.TokenPurpose) ([]domain.EmailToken, error)
	DeleteEmailToken(ctx context.Context, id domain.ID, purpose domain.TokenPurpose) error
}

type SettingsRepo interface {
	// LoadSettings returns the stored settings over the defaults.
	LoadSettings(ctx context.Context, defaults domain.InstanceSettings) (domain.InstanceSettings, error)
	SaveSettings(ctx context.Context, s domain.InstanceSettings) error
}

type GuildRepo interface {
	CreateGuild(ctx context.Context, g domain.Guild) error
	GuildByID(ctx context.Context, id domain.ID) (domain.Guild, error)
	UpdateGuild(ctx context.Context, g domain.Guild) error
	// DeleteGuild also deletes its channels, members, invites, bans and audit log.
	DeleteGuild(ctx context.Context, id domain.ID) error
	CountGuildsOwnedBy(ctx context.Context, userID domain.ID) (int, error)
	// UserGuilds returns the guilds the user is a member of, oldest membership first.
	UserGuilds(ctx context.Context, userID domain.ID) ([]domain.Guild, error)
}

type MemberRepo interface {
	AddMember(ctx context.Context, m domain.Member) error
	MemberByID(ctx context.Context, guildID, userID domain.ID) (domain.Member, error)
	UpdateMemberRole(ctx context.Context, guildID, userID domain.ID, role domain.GuildRole, at time.Time) error
	UpdateMemberVoiceModeration(ctx context.Context, guildID, userID domain.ID, serverMute, serverDeaf bool, at time.Time) error
	RemoveMember(ctx context.Context, guildID, userID domain.ID) error
	GuildMembers(ctx context.Context, guildID domain.ID) ([]domain.Member, error)
	UserMemberships(ctx context.Context, userID domain.ID) ([]domain.Member, error)
	CountMembers(ctx context.Context, guildID domain.ID) (int, error)
}

type ChannelRepo interface {
	CreateChannel(ctx context.Context, c domain.Channel) error
	ChannelByID(ctx context.Context, id domain.ID) (domain.Channel, error)
	// UpdateChannel writes every mutable field of c.
	UpdateChannel(ctx context.Context, c domain.Channel) error
	DeleteChannel(ctx context.Context, id domain.ID) error
	// GuildChannels returns the channels ordered by position.
	GuildChannels(ctx context.Context, guildID domain.ID) ([]domain.Channel, error)
	CountChannels(ctx context.Context, guildID domain.ID) (int, error)
}

type InviteRepo interface {
	CreateInvite(ctx context.Context, inv domain.Invite) error
	InviteByCode(ctx context.Context, code string) (domain.Invite, error)
	// UseInvite counts one use if the invite is still usable at now, and
	// fails with ErrNotFound otherwise, so concurrent accepts cannot exceed
	// the limit.
	UseInvite(ctx context.Context, code string, now time.Time) error
	RevokeInvite(ctx context.Context, code string, at time.Time) error
	// UsableInvites returns the guild invites that can still be accepted, newest first.
	UsableInvites(ctx context.Context, guildID domain.ID, now time.Time) ([]domain.Invite, error)
}

type BanRepo interface {
	CreateBan(ctx context.Context, b domain.Ban) error
	BanByID(ctx context.Context, guildID, userID domain.ID) (domain.Ban, error)
	DeleteBan(ctx context.Context, guildID, userID domain.ID) error
	GuildBans(ctx context.Context, guildID domain.ID) ([]domain.Ban, error)
}

type AuditRepo interface {
	AppendAudit(ctx context.Context, e domain.AuditEntry) error
	// AuditLog returns entries older than before (zero for the newest), newest first.
	AuditLog(ctx context.Context, guildID, before domain.ID, limit int) ([]domain.AuditEntry, error)
}

type FileRepo interface {
	CreateFile(ctx context.Context, f domain.File) error
	FileByID(ctx context.Context, id domain.ID) (domain.File, error)
}
