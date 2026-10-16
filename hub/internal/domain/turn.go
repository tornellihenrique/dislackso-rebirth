package domain

import (
	"regexp"
	"strings"
)

// TURNKind is where voice clients get relay servers from when a direct
// connection fails (ARCHITECTURE §4.3).
type TURNKind string

const (
	TURNNone       TURNKind = "none"
	TURNCloudflare TURNKind = "cloudflare"
	TURNStatic     TURNKind = "static"
	TURNCoturnREST TURNKind = "coturn-rest"
)

func (k TURNKind) Valid() bool {
	return k == TURNNone || k == TURNCloudflare || k == TURNStatic || k == TURNCoturnREST
}

// TURNSettings configure the relay provider. Its secret (Cloudflare API
// token, static password or coturn shared secret) lives in the secret store.
type TURNSettings struct {
	Kind TURNKind
	// CloudflareKeyID is the TURN key the hub mints credentials with.
	CloudflareKeyID string
	// URLs are the relay addresses for static and coturn-rest
	// ("turn:relay.example.com:3478?transport=udp").
	URLs []string
	// Username is the static provider's fixed username.
	Username string
	// BudgetOverride keeps issuing relay credentials past the monthly budget.
	BudgetOverride bool
}

// TURNBudgetMaxGB bounds the monthly relay budget an admin can set.
const TURNBudgetMaxGB = 100_000

// TURNUsage is how much went through relays this month.
type TURNUsage struct {
	MonthBytes     int64
	ProjectedBytes int64
	BudgetGB       int
	// OverBudget: relay credentials stop unless an admin overrides.
	OverBudget      bool
	VoiceSessions   int
	RelayedSessions int
}

var cloudflareKeyRe = regexp.MustCompile(`^[a-f0-9]{32}$`)

// ValidateTURN checks the settings the chosen provider needs.
func ValidateTURN(prefix string, s TURNSettings, secretSet bool) []*FieldViolation {
	if !s.Kind.Valid() {
		return []*FieldViolation{{prefix + "kind", ReasonFieldInvalid}}
	}
	var out []*FieldViolation
	needSecret := func() {
		if !secretSet {
			out = append(out, &FieldViolation{prefix + "secret", ReasonFieldRequired})
		}
	}
	switch s.Kind {
	case TURNCloudflare:
		if !cloudflareKeyRe.MatchString(s.CloudflareKeyID) {
			out = append(out, &FieldViolation{prefix + "cloudflare_key_id", ReasonFieldInvalidFormat})
		}
		needSecret()
	case TURNStatic, TURNCoturnREST:
		if len(s.URLs) == 0 {
			out = append(out, &FieldViolation{prefix + "urls", ReasonFieldRequired})
		}
		for _, u := range s.URLs {
			if !strings.HasPrefix(u, "turn:") && !strings.HasPrefix(u, "turns:") || hasControl(u) || len(u) > 512 {
				out = append(out, &FieldViolation{prefix + "urls", ReasonFieldInvalidFormat})
				break
			}
		}
		if s.Kind == TURNStatic && strings.TrimSpace(s.Username) == "" {
			out = append(out, &FieldViolation{prefix + "username", ReasonFieldRequired})
		}
		needSecret()
	case TURNNone:
	}
	return out
}
