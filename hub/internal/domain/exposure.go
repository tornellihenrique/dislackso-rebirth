package domain

import (
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ExposureKind is how the hub is reached from outside this machine
// (ARCHITECTURE §4.2).
type ExposureKind string

const (
	// ExposureLocal keeps the hub on this machine only.
	ExposureLocal            ExposureKind = "local"
	ExposureTailscaleFunnel  ExposureKind = "tailscale-funnel"
	ExposureCloudflareQuick  ExposureKind = "cloudflare-quick"
	ExposureCloudflareTunnel ExposureKind = "cloudflare-tunnel"
	ExposureDirect           ExposureKind = "direct"
	ExposureLAN              ExposureKind = "lan"
)

func (k ExposureKind) Valid() bool {
	switch k {
	case ExposureLocal, ExposureTailscaleFunnel, ExposureCloudflareQuick,
		ExposureCloudflareTunnel, ExposureDirect, ExposureLAN:
		return true
	}
	return false
}

// ExposureSettings choose and configure the exposure provider. Secrets (the
// Cloudflare tunnel token) live in the secret store.
type ExposureSettings struct {
	Kind ExposureKind
	// TailscaleHostname names the node: https://<hostname>.<tailnet>.ts.net.
	TailscaleHostname string
	// TunnelURL is the public hostname routed to the hub in the Cloudflare
	// dashboard; the token alone does not tell it.
	TunnelURL string
	// DirectDomain gets a certificate from Let's Encrypt; DirectEmail is the
	// ACME contact. DirectAddr is where the hub listens for it (":443").
	DirectDomain string
	DirectEmail  string
	DirectAddr   string
	// LANAddr is the address on this machine's network the hub listens on,
	// with a self-signed certificate.
	LANAddr string
}

const DefaultTailscaleHostname = "dislackso"

func DefaultExposureSettings() ExposureSettings {
	return ExposureSettings{
		Kind:              ExposureLocal,
		TailscaleHostname: DefaultTailscaleHostname,
		DirectAddr:        ":443",
		LANAddr:           "0.0.0.0:7443",
	}
}

// ExposureState is where an exposure provider stands.
type ExposureState string

const (
	ExposureOff        ExposureState = "off"
	ExposureStarting   ExposureState = "starting"
	ExposureNeedsLogin ExposureState = "needs_login"
	ExposureRunning    ExposureState = "running"
	ExposureFailed     ExposureState = "failed"
)

// Problems an exposure provider reports, so the admin panel can say what to do.
const (
	ProblemTailscaleHTTPS    = "tailscale_https_disabled"
	ProblemTailscaleFunnel   = "tailscale_funnel_not_allowed"
	ProblemTailscaleLogin    = "tailscale_login"
	ProblemCloudflaredFetch  = "cloudflared_download_failed"
	ProblemCloudflaredExited = "cloudflared_exited"
	ProblemTunnelToken       = "tunnel_token_missing"
	ProblemAddressInUse      = "address_in_use"
	ProblemCertificate       = "certificate_failed"
	ProblemExposureFailed    = "exposure_failed"
)

type ExposureStatus struct {
	Kind  ExposureKind
	State ExposureState
	// URL is the public address while running.
	URL string
	// LoginURL is where the host signs the hub in (Tailscale).
	LoginURL string
	// Problem is one of the Problem* codes; Detail is the error text.
	Problem string
	Detail  string
	Since   time.Time
}

// Hostname labels: letters, digits and dashes, as Tailscale accepts them.
var hostnameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// ValidateExposure checks the settings the chosen provider needs.
func ValidateExposure(prefix string, s ExposureSettings, tunnelTokenSet bool) []*FieldViolation {
	var out []*FieldViolation
	if !s.Kind.Valid() {
		return []*FieldViolation{{prefix + "kind", ReasonFieldInvalid}}
	}
	switch s.Kind {
	case ExposureTailscaleFunnel:
		if !hostnameRe.MatchString(s.TailscaleHostname) {
			out = append(out, &FieldViolation{prefix + "tailscale_hostname", ReasonFieldInvalidFormat})
		}
	case ExposureCloudflareTunnel:
		if !tunnelTokenSet {
			out = append(out, &FieldViolation{prefix + "tunnel_token", ReasonFieldRequired})
		}
		if v := validateHTTPSURL(prefix+"tunnel_url", s.TunnelURL); v != nil {
			out = append(out, v)
		}
	case ExposureDirect:
		if !validDomain(s.DirectDomain) {
			out = append(out, &FieldViolation{prefix + "direct_domain", ReasonFieldInvalidFormat})
		}
		if s.DirectEmail != "" {
			out = append(out, ValidateEmail(prefix+"direct_email", s.DirectEmail))
		}
		if !validListenAddr(s.DirectAddr) {
			out = append(out, &FieldViolation{prefix + "direct_addr", ReasonFieldInvalidFormat})
		}
	case ExposureLAN:
		if !validListenAddr(s.LANAddr) {
			out = append(out, &FieldViolation{prefix + "lan_addr", ReasonFieldInvalidFormat})
		}
	case ExposureLocal, ExposureCloudflareQuick:
	}
	return out
}

func validateHTTPSURL(field, raw string) *FieldViolation {
	if raw == "" {
		return &FieldViolation{field, ReasonFieldRequired}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || (u.Path != "" && u.Path != "/") ||
		u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return &FieldViolation{field, ReasonFieldInvalidFormat}
	}
	return nil
}

func validDomain(d string) bool {
	labels := strings.Split(d, ".")
	if len(labels) < 2 || len(d) > 253 {
		return false
	}
	for _, l := range labels {
		if !hostnameRe.MatchString(l) {
			return false
		}
	}
	return true
}

func validListenAddr(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host != "" && net.ParseIP(host) == nil {
		return false
	}
	p, err := strconv.Atoi(port)
	return err == nil && p > 0 && p < 65536
}
