package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/sqlite/sqlcgen"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

// Settings are stored as one JSON document per section, so adding a field
// needs no migration and missing fields keep their defaults.
const (
	sectionInstance     = "instance"
	sectionRegistration = "registration"
	sectionSMTP         = "smtp"
	sectionLimits       = "limits"
)

type instanceJSON struct {
	Name             string `json:"name"`
	PublicURL        string `json:"public_url"`
	SetupCompletedAt *int64 `json:"setup_completed_at,omitempty"`
}

type registrationJSON struct {
	Mode             string `json:"mode"`
	MembersCanInvite bool   `json:"members_can_invite"`
}

type smtpJSON struct {
	Enabled     bool   `json:"enabled"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Security    string `json:"security"`
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
}

type limitsJSON struct {
	MaxGuildsOwnedPerUser int    `json:"max_guilds_owned_per_user"`
	MaxGuildMembers       int    `json:"max_guild_members"`
	MaxChannelsPerGuild   int    `json:"max_channels_per_guild"`
	DefaultVoiceUserLimit int    `json:"default_voice_user_limit"`
	MaxSharesPerChannel   int    `json:"max_shares_per_channel"`
	MaxSharePreset        string `json:"max_share_preset"`
	MaxUploadMB           int    `json:"max_upload_mb"`
	StrokeHoldMaxMS       int    `json:"stroke_hold_max_ms"`
	MicBitrateMaxKbps     int    `json:"mic_bitrate_max_kbps"`
	TURNMonthlyBudgetGB   int    `json:"turn_monthly_budget_gb"`
}

func (db *DB) LoadSettings(ctx context.Context, defaults domain.InstanceSettings) (domain.InstanceSettings, error) {
	rows, err := db.reader(ctx).ListSettings(ctx)
	if err != nil {
		return domain.InstanceSettings{}, fmt.Errorf("load settings: %w", mapError(err))
	}
	inst, reg, smtp, limits := sections(defaults)
	targets := map[string]any{
		sectionInstance:     &inst,
		sectionRegistration: &reg,
		sectionSMTP:         &smtp,
		sectionLimits:       &limits,
	}
	for _, r := range rows {
		target, ok := targets[r.Key]
		if !ok {
			continue
		}
		if err := json.Unmarshal([]byte(r.ValueJson), target); err != nil {
			return domain.InstanceSettings{}, fmt.Errorf("decode setting %q: %w", r.Key, err)
		}
	}
	return fromSections(inst, reg, smtp, limits), nil
}

func (db *DB) SaveSettings(ctx context.Context, s domain.InstanceSettings) error {
	inst, reg, smtp, limits := sections(s)
	values := []struct {
		key   string
		value any
	}{
		{sectionInstance, inst},
		{sectionRegistration, reg},
		{sectionSMTP, smtp},
		{sectionLimits, limits},
	}
	now := toMS(time.Now())
	return db.InTx(ctx, func(ctx context.Context) error {
		for _, v := range values {
			data, err := json.Marshal(v.value)
			if err != nil {
				return fmt.Errorf("encode setting %q: %w", v.key, err)
			}
			err = db.writer(ctx).UpsertSetting(ctx, sqlcgen.UpsertSettingParams{
				Key:       v.key,
				ValueJson: string(data),
				UpdatedAt: now,
			})
			if err != nil {
				return fmt.Errorf("save setting %q: %w", v.key, mapError(err))
			}
		}
		return nil
	})
}

func sections(s domain.InstanceSettings) (instanceJSON, registrationJSON, smtpJSON, limitsJSON) {
	var completed *int64
	if s.SetupCompletedAt != nil {
		ms := toMS(*s.SetupCompletedAt)
		completed = &ms
	}
	return instanceJSON{
		Name:             s.Name,
		PublicURL:        s.PublicURL,
		SetupCompletedAt: completed,
	}, registrationJSON{
		Mode:             string(s.RegistrationMode),
		MembersCanInvite: s.MembersCanInvite,
	}, smtpJSON{
		Enabled:     s.SMTP.Enabled,
		Host:        s.SMTP.Host,
		Port:        s.SMTP.Port,
		Username:    s.SMTP.Username,
		Security:    string(s.SMTP.Security),
		FromAddress: s.SMTP.FromAddress,
		FromName:    s.SMTP.FromName,
	}, limitsJSON(s.Limits)
}

func fromSections(inst instanceJSON, reg registrationJSON, smtp smtpJSON, limits limitsJSON) domain.InstanceSettings {
	var completed *time.Time
	if inst.SetupCompletedAt != nil {
		t := fromMS(*inst.SetupCompletedAt)
		completed = &t
	}
	return domain.InstanceSettings{
		Name:             inst.Name,
		PublicURL:        inst.PublicURL,
		SetupCompletedAt: completed,
		RegistrationMode: domain.RegistrationMode(reg.Mode),
		MembersCanInvite: reg.MembersCanInvite,
		SMTP: domain.SMTPSettings{
			Enabled:     smtp.Enabled,
			Host:        smtp.Host,
			Port:        smtp.Port,
			Username:    smtp.Username,
			Security:    domain.SMTPSecurity(smtp.Security),
			FromAddress: smtp.FromAddress,
			FromName:    smtp.FromName,
		},
		Limits: domain.Limits(limits),
	}
}
