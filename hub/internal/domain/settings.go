package domain

import (
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

type RegistrationMode string

const (
	RegistrationInviteOnly RegistrationMode = "invite_only"
	RegistrationOpen       RegistrationMode = "open"
	RegistrationClosed     RegistrationMode = "closed"
)

func (m RegistrationMode) Valid() bool {
	return m == RegistrationInviteOnly || m == RegistrationOpen || m == RegistrationClosed
}

type SMTPSecurity string

const (
	SMTPStartTLS SMTPSecurity = "starttls"
	SMTPTLS      SMTPSecurity = "tls"
	SMTPNone     SMTPSecurity = "none"
)

func (s SMTPSecurity) Valid() bool { return s == SMTPStartTLS || s == SMTPTLS || s == SMTPNone }

// SMTPSettings is the outgoing mail server. The password is kept in the
// secret store, never in the database.
type SMTPSettings struct {
	Enabled     bool
	Host        string
	Port        int
	Username    string
	Security    SMTPSecurity
	FromAddress string
	FromName    string
}

// Limits are the instance-wide caps from ARCHITECTURE §13.1.
type Limits struct {
	MaxGuildsOwnedPerUser int
	MaxGuildMembers       int
	MaxChannelsPerGuild   int
	DefaultVoiceUserLimit int
	MaxSharesPerChannel   int
	MaxSharePreset        string
	MaxUploadMB           int
	StrokeHoldMaxMS       int
	MicBitrateMaxKbps     int
	TURNMonthlyBudgetGB   int
}

type InstanceSettings struct {
	Name             string
	PublicURL        string
	SetupCompletedAt *time.Time
	RegistrationMode RegistrationMode
	MembersCanInvite bool
	SMTP             SMTPSettings
	Limits           Limits
	Exposure         ExposureSettings
	TURN             TURNSettings
	Host             HostSettings
}

// HostSettings are conveniences for the machine running the hub.
type HostSettings struct {
	// KeepAwake stops the machine from sleeping while the hub runs.
	KeepAwake bool
}

func (s InstanceSettings) SetupComplete() bool { return s.SetupCompletedAt != nil }

func DefaultInstanceSettings() InstanceSettings {
	return InstanceSettings{
		RegistrationMode: RegistrationInviteOnly,
		MembersCanInvite: true,
		SMTP:             SMTPSettings{Port: 587, Security: SMTPStartTLS},
		Limits: Limits{
			MaxGuildsOwnedPerUser: 5,
			MaxGuildMembers:       200,
			MaxChannelsPerGuild:   50,
			DefaultVoiceUserLimit: 25,
			MaxSharesPerChannel:   6,
			MaxSharePreset:        "1440p60",
			MaxUploadMB:           100,
			StrokeHoldMaxMS:       10_000,
			MicBitrateMaxKbps:     96,
			TURNMonthlyBudgetGB:   900,
		},
		Exposure: DefaultExposureSettings(),
		TURN:     TURNSettings{Kind: TURNNone},
		Host:     HostSettings{KeepAwake: true},
	}
}

const InstanceNameMaxLen = 64

func ValidateInstanceName(field, name string) *FieldViolation {
	switch n := utf8.RuneCountInString(name); {
	case n == 0:
		return &FieldViolation{field, ReasonFieldRequired}
	case n > InstanceNameMaxLen:
		return &FieldViolation{field, ReasonFieldTooLong}
	}
	if hasControl(name) {
		return &FieldViolation{field, ReasonFieldInvalidFormat}
	}
	return nil
}

// NormalizePublicURL trims space and trailing slashes.
func NormalizePublicURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

// ValidatePublicURL requires an absolute http(s) URL without query or fragment.
func ValidatePublicURL(field, raw string) *FieldViolation {
	if raw == "" {
		return &FieldViolation{field, ReasonFieldRequired}
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return &FieldViolation{field, ReasonFieldInvalidFormat}
	}
	return nil
}

// ValidateSMTP checks the settings only when they are enabled; a disabled
// configuration may be left half-filled.
func ValidateSMTP(prefix string, s SMTPSettings) []*FieldViolation {
	if !s.Enabled {
		return nil
	}
	var out []*FieldViolation
	if strings.TrimSpace(s.Host) == "" {
		out = append(out, &FieldViolation{prefix + "host", ReasonFieldRequired})
	}
	if s.Port < 1 || s.Port > 65535 {
		out = append(out, &FieldViolation{prefix + "port", ReasonFieldInvalidFormat})
	}
	if !s.Security.Valid() {
		out = append(out, &FieldViolation{prefix + "security", ReasonFieldInvalidFormat})
	}
	out = append(out, ValidateEmail(prefix+"from_address", s.FromAddress))
	if utf8.RuneCountInString(s.FromName) > InstanceNameMaxLen || hasControl(s.FromName) {
		out = append(out, &FieldViolation{prefix + "from_name", ReasonFieldInvalidFormat})
	}
	return out
}
