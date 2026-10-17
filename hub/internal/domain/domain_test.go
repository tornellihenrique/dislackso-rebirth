package domain

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"
)

func reasonOf(v *FieldViolation) string {
	if v == nil {
		return ""
	}
	return v.Reason
}

func TestValidators(t *testing.T) {
	tests := []struct {
		name string
		got  *FieldViolation
		want string
	}{
		{"username ok", ValidateUsername("u", "ana.b_9"), ""},
		{"username empty", ValidateUsername("u", ""), ReasonFieldRequired},
		{"username short", ValidateUsername("u", "ab"), ReasonFieldTooShort},
		{"username long", ValidateUsername("u", strings.Repeat("a", 33)), ReasonFieldTooLong},
		{"username space", ValidateUsername("u", "ana b"), ReasonFieldInvalidFormat},
		{"username non-ascii", ValidateUsername("u", "anaé"), ReasonFieldInvalidFormat},
		{"email ok", ValidateEmail("e", "ana@example.com"), ""},
		{"email empty", ValidateEmail("e", ""), ReasonFieldRequired},
		{"email with name", ValidateEmail("e", "Ana <ana@example.com>"), ReasonFieldInvalidFormat},
		{"email no at", ValidateEmail("e", "ana.example.com"), ReasonFieldInvalidFormat},
		{"email too long", ValidateEmail("e", strings.Repeat("a", 250)+"@x.io"), ReasonFieldTooLong},
		{"password ok", ValidatePassword("p", "correct horse"), ""},
		{"password empty", ValidatePassword("p", ""), ReasonFieldRequired},
		{"password short", ValidatePassword("p", "seven77"), ReasonFieldTooShort},
		{"password counts runes", ValidatePassword("p", "çççççççç"), ""},
		{"password common", ValidatePassword("p", "Password123"), ReasonFieldTooCommon},
		{"password long", ValidatePassword("p", strings.Repeat("x", 257)), ReasonFieldTooLong},
		{"display name ok", ValidateDisplayName("d", "Ana Beatriz 🎧"), ""},
		{"display name empty", ValidateDisplayName("d", ""), ReasonFieldRequired},
		{"display name control", ValidateDisplayName("d", "a\nb"), ReasonFieldInvalidFormat},
		{"display name long", ValidateDisplayName("d", strings.Repeat("é", 33)), ReasonFieldTooLong},
		{"custom status long", ValidateCustomStatus("c", strings.Repeat("x", 129)), ReasonFieldTooLong},
		{"instance name ok", ValidateInstanceName("n", "Team"), ""},
		{"public url ok", ValidatePublicURL("u", "https://hub.example.ts.net"), ""},
		{"public url with path", ValidatePublicURL("u", "https://example.com/hub"), ""},
		{"public url scheme", ValidatePublicURL("u", "ftp://example.com"), ReasonFieldInvalidFormat},
		{"public url userinfo", ValidatePublicURL("u", "https://user@example.com"), ReasonFieldInvalidFormat},
		{"public url fragment", ValidatePublicURL("u", "https://example.com#x"), ReasonFieldInvalidFormat},
	}
	for _, tt := range tests {
		if got := reasonOf(tt.got); got != tt.want {
			t.Errorf("%s: reason = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestValidateSMTP(t *testing.T) {
	if got := ValidateSMTP("smtp.", SMTPSettings{}); len(got) != 0 {
		t.Errorf("disabled settings must not be validated: %v", got)
	}
	err := Violations(ValidateSMTP("smtp.", SMTPSettings{Enabled: true, Port: 70000, Security: "ssl", FromAddress: "x"})...)
	de, ok := AsError(err)
	if !ok {
		t.Fatalf("err = %v", err)
	}
	got := map[string]bool{}
	for _, f := range de.Fields {
		got[f.Field] = true
	}
	for _, f := range []string{"smtp.host", "smtp.port", "smtp.security", "smtp.from_address"} {
		if !got[f] {
			t.Errorf("missing violation for %s (got %v)", f, de.Fields)
		}
	}
}

func TestValidateExposureDirect(t *testing.T) {
	direct := func(domain string, dns DynamicDNSKind) ExposureSettings {
		s := DefaultExposureSettings()
		s.Kind, s.DirectDomain, s.DynamicDNS = ExposureDirect, domain, dns
		return s
	}
	tests := []struct {
		name        string
		settings    ExposureSettings
		dnsTokenSet bool
		want        []string
	}{
		{"own domain", direct("hub.example.com", DynamicDNSNone), false, nil},
		{"duckdns", direct("team.duckdns.org", DynamicDNSDuckDNS), true, nil},
		{"duckdns without token", direct("team.duckdns.org", DynamicDNSDuckDNS), false, []string{"e.dns_token"}},
		{"duckdns on another domain", direct("hub.example.com", DynamicDNSDuckDNS), true, []string{"e.direct_domain"}},
		{"duckdns nested name", direct("a.team.duckdns.org", DynamicDNSDuckDNS), true, []string{"e.direct_domain"}},
		{"empty domain once", direct("", DynamicDNSDuckDNS), true, []string{"e.direct_domain"}},
		{"unknown provider", direct("hub.example.com", "noip"), false, []string{"e.dynamic_dns"}},
	}
	for _, tt := range tests {
		var got []string
		for _, v := range ValidateExposure("e.", tt.settings, false, tt.dnsTokenSet) {
			got = append(got, v.Field)
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s: violations = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestDuckDNSName(t *testing.T) {
	if name, ok := DuckDNSName("team.duckdns.org"); !ok || name != "team" {
		t.Errorf("DuckDNSName = %q, %v", name, ok)
	}
	if _, ok := DuckDNSName("duckdns.org"); ok {
		t.Error("the bare suffix is not a DuckDNS name")
	}
}

func TestErrors(t *testing.T) {
	err := fmt.Errorf("register: %w", Invalid(FieldViolation{"email", ReasonFieldTaken}))
	if !errors.Is(err, ErrInvalid) || errors.Is(err, ErrConflict) {
		t.Errorf("kind detection failed: %v", err)
	}
	de, ok := AsError(err)
	if !ok || de.Fields[0].Field != "email" {
		t.Fatalf("AsError = %+v, %v", de, ok)
	}
	if got := RateLimited(time.Second).Error(); got != "rate limited" {
		t.Errorf("Error() = %q", got)
	}
	if Violations(nil, nil) != nil {
		t.Error("no violations must give a nil error")
	}
}

func TestParseID(t *testing.T) {
	if id, err := ParseID("123456789012345678", "id"); err != nil || id != 123456789012345678 {
		t.Errorf("ParseID = %d, %v", id, err)
	}
	for _, s := range []string{"", "0", "-5", "abc", "1.5", "99999999999999999999"} {
		if _, err := ParseID(s, "id"); !errors.Is(err, ErrInvalid) {
			t.Errorf("ParseID(%q) err = %v, want invalid", s, err)
		}
	}
}

func TestClientIsLocal(t *testing.T) {
	tests := map[string]bool{"127.0.0.1": true, "::1": true, "::ffff:127.0.0.1": true, "192.168.1.5": false, "203.0.113.1": false}
	for addr, want := range tests {
		if got := (ClientInfo{IP: netip.MustParseAddr(addr)}).IsLocal(); got != want {
			t.Errorf("IsLocal(%s) = %v, want %v", addr, got, want)
		}
	}
	if (ClientInfo{}).IsLocal() {
		t.Error("an unknown address must not count as local")
	}
}

func TestPolicyManageInstance(t *testing.T) {
	var p Policy
	if err := p.ManageInstance(Actor{InstanceAdmin: true}); err != nil {
		t.Errorf("admin refused: %v", err)
	}
	if err := p.ManageInstance(Actor{}); !errors.Is(err, ErrForbidden) {
		t.Errorf("member allowed: %v", err)
	}
}
