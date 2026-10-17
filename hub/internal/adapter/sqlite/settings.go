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
	sectionExposure     = "exposure"
	sectionTURN         = "turn"
	sectionHost         = "host"
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

type exposureJSON struct {
	Kind              string `json:"kind"`
	TailscaleHostname string `json:"tailscale_hostname"`
	TunnelURL         string `json:"tunnel_url"`
	DirectDomain      string `json:"direct_domain"`
	DirectEmail       string `json:"direct_email"`
	DirectAddr        string `json:"direct_addr"`
	DynamicDNS        string `json:"dynamic_dns"`
	LANAddr           string `json:"lan_addr"`
}

type turnJSON struct {
	Kind            string   `json:"kind"`
	CloudflareKeyID string   `json:"cloudflare_key_id"`
	URLs            []string `json:"urls"`
	Username        string   `json:"username"`
	BudgetOverride  bool     `json:"budget_override"`
}

type hostJSON struct {
	KeepAwake bool `json:"keep_awake"`
}

type settingsJSON struct {
	instance     instanceJSON
	registration registrationJSON
	smtp         smtpJSON
	limits       limitsJSON
	exposure     exposureJSON
	turn         turnJSON
	host         hostJSON
}

func (s *settingsJSON) targets() map[string]any {
	return map[string]any{
		sectionInstance:     &s.instance,
		sectionRegistration: &s.registration,
		sectionSMTP:         &s.smtp,
		sectionLimits:       &s.limits,
		sectionExposure:     &s.exposure,
		sectionTURN:         &s.turn,
		sectionHost:         &s.host,
	}
}

func (db *DB) LoadSettings(ctx context.Context, defaults domain.InstanceSettings) (domain.InstanceSettings, error) {
	rows, err := db.reader(ctx).ListSettings(ctx)
	if err != nil {
		return domain.InstanceSettings{}, fmt.Errorf("load settings: %w", mapError(err))
	}
	sec := sections(defaults)
	targets := sec.targets()
	for _, r := range rows {
		target, ok := targets[r.Key]
		if !ok {
			continue
		}
		if err := json.Unmarshal([]byte(r.ValueJson), target); err != nil {
			return domain.InstanceSettings{}, fmt.Errorf("decode setting %q: %w", r.Key, err)
		}
	}
	return sec.settings(), nil
}

func (db *DB) SaveSettings(ctx context.Context, s domain.InstanceSettings) error {
	sec := sections(s)
	now := toMS(time.Now())
	return db.InTx(ctx, func(ctx context.Context) error {
		for key, value := range sec.targets() {
			data, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("encode setting %q: %w", key, err)
			}
			err = db.writer(ctx).UpsertSetting(ctx, sqlcgen.UpsertSettingParams{
				Key:       key,
				ValueJson: string(data),
				UpdatedAt: now,
			})
			if err != nil {
				return fmt.Errorf("save setting %q: %w", key, mapError(err))
			}
		}
		return nil
	})
}

func sections(s domain.InstanceSettings) settingsJSON {
	var completed *int64
	if s.SetupCompletedAt != nil {
		ms := toMS(*s.SetupCompletedAt)
		completed = &ms
	}
	e, t := s.Exposure, s.TURN
	return settingsJSON{
		instance: instanceJSON{
			Name:             s.Name,
			PublicURL:        s.PublicURL,
			SetupCompletedAt: completed,
		},
		registration: registrationJSON{
			Mode:             string(s.RegistrationMode),
			MembersCanInvite: s.MembersCanInvite,
		},
		smtp: smtpJSON{
			Enabled:     s.SMTP.Enabled,
			Host:        s.SMTP.Host,
			Port:        s.SMTP.Port,
			Username:    s.SMTP.Username,
			Security:    string(s.SMTP.Security),
			FromAddress: s.SMTP.FromAddress,
			FromName:    s.SMTP.FromName,
		},
		limits: limitsJSON(s.Limits),
		exposure: exposureJSON{
			Kind:              string(e.Kind),
			TailscaleHostname: e.TailscaleHostname,
			TunnelURL:         e.TunnelURL,
			DirectDomain:      e.DirectDomain,
			DirectEmail:       e.DirectEmail,
			DirectAddr:        e.DirectAddr,
			DynamicDNS:        string(e.DynamicDNS),
			LANAddr:           e.LANAddr,
		},
		turn: turnJSON{
			Kind:            string(t.Kind),
			CloudflareKeyID: t.CloudflareKeyID,
			URLs:            t.URLs,
			Username:        t.Username,
			BudgetOverride:  t.BudgetOverride,
		},
		host: hostJSON{KeepAwake: s.Host.KeepAwake},
	}
}

func (s settingsJSON) settings() domain.InstanceSettings {
	var completed *time.Time
	if s.instance.SetupCompletedAt != nil {
		t := fromMS(*s.instance.SetupCompletedAt)
		completed = &t
	}
	e, t := s.exposure, s.turn
	return domain.InstanceSettings{
		Name:             s.instance.Name,
		PublicURL:        s.instance.PublicURL,
		SetupCompletedAt: completed,
		RegistrationMode: domain.RegistrationMode(s.registration.Mode),
		MembersCanInvite: s.registration.MembersCanInvite,
		SMTP: domain.SMTPSettings{
			Enabled:     s.smtp.Enabled,
			Host:        s.smtp.Host,
			Port:        s.smtp.Port,
			Username:    s.smtp.Username,
			Security:    domain.SMTPSecurity(s.smtp.Security),
			FromAddress: s.smtp.FromAddress,
			FromName:    s.smtp.FromName,
		},
		Limits: domain.Limits(s.limits),
		Exposure: domain.ExposureSettings{
			Kind:              domain.ExposureKind(e.Kind),
			TailscaleHostname: e.TailscaleHostname,
			TunnelURL:         e.TunnelURL,
			DirectDomain:      e.DirectDomain,
			DirectEmail:       e.DirectEmail,
			DirectAddr:        e.DirectAddr,
			DynamicDNS:        dynamicDNS(e.DynamicDNS),
			LANAddr:           e.LANAddr,
		},
		TURN: domain.TURNSettings{
			Kind:            domain.TURNKind(t.Kind),
			CloudflareKeyID: t.CloudflareKeyID,
			URLs:            t.URLs,
			Username:        t.Username,
			BudgetOverride:  t.BudgetOverride,
		},
		Host: domain.HostSettings{KeepAwake: s.host.KeepAwake},
	}
}

// dynamicDNS reads settings saved before the field existed as none.
func dynamicDNS(raw string) domain.DynamicDNSKind {
	if raw == "" {
		return domain.DynamicDNSNone
	}
	return domain.DynamicDNSKind(raw)
}
