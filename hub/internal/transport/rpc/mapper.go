package rpc

import (
	"math"
	"time"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/instance"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/invite"
)

// toPublicUser is the only place a user becomes visible to other users;
// any field added here becomes public.
func toPublicUser(u domain.User) *dislacksov1.User {
	return &dislacksov1.User{
		Id:          u.ID.String(),
		Username:    u.Username,
		DisplayName: u.DisplayName,
	}
}

func toMe(u domain.User) *dislacksov1.Me {
	return &dislacksov1.Me{
		User:          toPublicUser(u),
		Email:         u.Email,
		EmailVerified: u.EmailVerified(),
		Status:        statusToProto(u.StatusPref),
		CustomStatus:  u.CustomStatus,
		InstanceAdmin: u.IsInstanceAdmin,
		CreatedAt:     u.CreatedAt.UnixMilli(),
	}
}

var statuses = map[domain.StatusPref]dislacksov1.StatusPreference{
	domain.StatusOnline:    dislacksov1.StatusPreference_STATUS_PREFERENCE_ONLINE,
	domain.StatusIdle:      dislacksov1.StatusPreference_STATUS_PREFERENCE_IDLE,
	domain.StatusDND:       dislacksov1.StatusPreference_STATUS_PREFERENCE_DND,
	domain.StatusInvisible: dislacksov1.StatusPreference_STATUS_PREFERENCE_INVISIBLE,
}

func statusToProto(s domain.StatusPref) dislacksov1.StatusPreference { return statuses[s] }

// statusFromProto maps unknown values to an invalid preference the service rejects.
func statusFromProto(s dislacksov1.StatusPreference) domain.StatusPref {
	for k, v := range statuses {
		if v == s {
			return k
		}
	}
	return ""
}

var registrationModes = map[domain.RegistrationMode]dislacksov1.RegistrationMode{
	domain.RegistrationInviteOnly: dislacksov1.RegistrationMode_REGISTRATION_MODE_INVITE_ONLY,
	domain.RegistrationOpen:       dislacksov1.RegistrationMode_REGISTRATION_MODE_OPEN,
	domain.RegistrationClosed:     dislacksov1.RegistrationMode_REGISTRATION_MODE_CLOSED,
}

func registrationModeToProto(m domain.RegistrationMode) dislacksov1.RegistrationMode {
	return registrationModes[m]
}

func registrationModeFromProto(m dislacksov1.RegistrationMode) domain.RegistrationMode {
	for k, v := range registrationModes {
		if v == m {
			return k
		}
	}
	return ""
}

var smtpSecurities = map[domain.SMTPSecurity]dislacksov1.SmtpSecurity{
	domain.SMTPStartTLS: dislacksov1.SmtpSecurity_SMTP_SECURITY_STARTTLS,
	domain.SMTPTLS:      dislacksov1.SmtpSecurity_SMTP_SECURITY_TLS,
	domain.SMTPNone:     dislacksov1.SmtpSecurity_SMTP_SECURITY_NONE,
}

func smtpToProto(s domain.SMTPSettings, passwordSet bool) *dislacksov1.SmtpSettings {
	return &dislacksov1.SmtpSettings{
		Enabled:     s.Enabled,
		Host:        s.Host,
		Port:        clampUint32(s.Port),
		Username:    s.Username,
		PasswordSet: passwordSet,
		Security:    smtpSecurities[s.Security],
		FromAddress: s.FromAddress,
		FromName:    s.FromName,
	}
}

// smtpFromProto returns the settings and the write-only password.
func smtpFromProto(p *dislacksov1.SmtpSettings) (domain.SMTPSettings, string) {
	var security domain.SMTPSecurity
	for k, v := range smtpSecurities {
		if v == p.GetSecurity() {
			security = k
		}
	}
	return domain.SMTPSettings{
		Enabled:     p.GetEnabled(),
		Host:        p.GetHost(),
		Port:        int(p.GetPort()),
		Username:    p.GetUsername(),
		Security:    security,
		FromAddress: p.GetFromAddress(),
		FromName:    p.GetFromName(),
	}, p.GetPassword()
}

func settingsToProto(v instance.SettingsView) *dislacksov1.InstanceSettings {
	s, l := v.Settings, v.Settings.Limits
	return &dislacksov1.InstanceSettings{
		Name:             s.Name,
		PublicUrl:        s.PublicURL,
		RegistrationMode: registrationModeToProto(s.RegistrationMode),
		MembersCanInvite: s.MembersCanInvite,
		Smtp:             smtpToProto(s.SMTP, v.SMTPPasswordSet),
		Limits: &dislacksov1.InstanceLimits{
			MaxGuildsOwnedPerUser: clampInt32(l.MaxGuildsOwnedPerUser),
			MaxGuildMembers:       clampInt32(l.MaxGuildMembers),
			MaxChannelsPerGuild:   clampInt32(l.MaxChannelsPerGuild),
			DefaultVoiceUserLimit: clampInt32(l.DefaultVoiceUserLimit),
			MaxSharesPerChannel:   clampInt32(l.MaxSharesPerChannel),
			MaxSharePreset:        l.MaxSharePreset,
			MaxUploadMb:           clampInt32(l.MaxUploadMB),
			StrokeHoldMaxMs:       clampInt32(l.StrokeHoldMaxMS),
			MicBitrateMaxKbps:     clampInt32(l.MicBitrateMaxKbps),
			TurnMonthlyBudgetGb:   clampInt32(l.TURNMonthlyBudgetGB),
		},
	}
}

func instanceUserToProto(u domain.User) *dislacksov1.InstanceUser {
	return &dislacksov1.InstanceUser{
		User:          toPublicUser(u),
		Email:         u.Email,
		EmailVerified: u.EmailVerified(),
		InstanceAdmin: u.IsInstanceAdmin,
		Disabled:      u.Disabled(),
		CreatedAt:     u.CreatedAt.UnixMilli(),
	}
}

func inviteToProto(inv invite.InstanceInvite) *dislacksov1.InstanceInvite {
	out := &dislacksov1.InstanceInvite{
		Id:        inv.Token.ID.String(),
		CreatedAt: inv.Token.CreatedAt.UnixMilli(),
		ExpiresAt: inv.Token.ExpiresAt.UnixMilli(),
		UsedAt:    unixMilliOrZero(inv.Token.UsedAt),
	}
	if inv.CreatedBy != nil {
		out.CreatedBy = toPublicUser(*inv.CreatedBy)
	}
	return out
}

func sessionToProto(s domain.Session, current domain.ID) *dislacksov1.Session {
	return &dislacksov1.Session{
		Id:         s.ID.String(),
		CreatedAt:  s.CreatedAt.UnixMilli(),
		LastSeenAt: s.LastSeenAt.UnixMilli(),
		UserAgent:  s.UserAgent,
		Ip:         s.IP,
		Current:    s.ID == current,
	}
}

func unixMilliOrZero(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.UnixMilli()
}

func clampInt32(v int) int32 {
	switch {
	case v > math.MaxInt32:
		return math.MaxInt32
	case v < math.MinInt32:
		return math.MinInt32
	default:
		return int32(v)
	}
}

func clampUint32(v int) uint32 {
	switch {
	case v < 0:
		return 0
	case v > math.MaxUint32:
		return math.MaxUint32
	default:
		return uint32(v)
	}
}
