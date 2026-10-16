// Package ice issues the ICE servers voice clients get on join.
package ice

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // coturn's REST credentials are defined as HMAC-SHA1
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

// None issues nothing: clients use only the media server's own STUN-derived
// candidates.
type None struct{}

func (None) Name() string { return "none" }

func (None) Issue(context.Context, domain.ID, time.Duration) ([]ports.IceServer, error) {
	return nil, nil
}

// Static issues one TURN server with fixed credentials, as free TURN
// services that hand out a single username and password expect.
type Static struct {
	URLs       []string
	Username   string
	Credential string
}

func (Static) Name() string { return "static" }

func (s Static) Issue(context.Context, domain.ID, time.Duration) ([]ports.IceServer, error) {
	if len(s.URLs) == 0 {
		return nil, nil
	}
	return []ports.IceServer{{URLs: slices.Clone(s.URLs), Username: s.Username, Credential: s.Credential}}, nil
}

// CoturnREST mints time-limited credentials for a coturn server that shares
// a secret with the hub (coturn's use-auth-secret).
type CoturnREST struct {
	URLs   []string
	Secret string
	Now    func() time.Time
}

func (CoturnREST) Name() string { return "coturn-rest" }

func (c CoturnREST) Issue(_ context.Context, userID domain.ID, ttl time.Duration) ([]ports.IceServer, error) {
	if len(c.URLs) == 0 || c.Secret == "" {
		return nil, nil
	}
	username := strconv.FormatInt(c.Now().Add(ttl).Unix(), 10) + ":" + userID.String()
	mac := hmac.New(sha1.New, []byte(c.Secret))
	_, _ = mac.Write([]byte(username))
	return []ports.IceServer{{
		URLs: slices.Clone(c.URLs), Username: username,
		Credential: base64.StdEncoding.EncodeToString(mac.Sum(nil)),
	}}, nil
}

// CloudflareAPI is where Cloudflare Realtime TURN mints credentials.
const CloudflareAPI = "https://rtc.live.cloudflare.com/v1/turn/keys/"

const maxCloudflareResponse = 64 << 10

// Cloudflare mints short-lived credentials from a Cloudflare Realtime TURN key.
type Cloudflare struct {
	KeyID    string
	APIToken string
	Client   *http.Client
	// BaseURL overrides CloudflareAPI, for tests.
	BaseURL string
}

func (Cloudflare) Name() string { return "cloudflare" }

type cloudflareServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username"`
	Credential string   `json:"credential"`
}

func (c Cloudflare) Issue(ctx context.Context, _ domain.ID, ttl time.Duration) ([]ports.IceServer, error) {
	base := c.BaseURL
	if base == "" {
		base = CloudflareAPI
	}
	body, err := json.Marshal(map[string]int64{"ttl": int64(ttl / time.Second)})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+c.KeyID+"/credentials/generate-ice-servers", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	req.Header.Set("Content-Type", "application/json")
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloudflare turn: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxCloudflareResponse))
	if err != nil {
		return nil, fmt.Errorf("cloudflare turn: %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("cloudflare turn: %s", resp.Status)
	}
	var out struct {
		IceServers json.RawMessage `json:"iceServers"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("cloudflare turn: %w", err)
	}
	var servers []cloudflareServer
	if err := json.Unmarshal(out.IceServers, &servers); err != nil {
		// Older keys answer with a single server object.
		var one cloudflareServer
		if json.Unmarshal(out.IceServers, &one) != nil {
			return nil, errors.New("cloudflare turn: unexpected response")
		}
		servers = []cloudflareServer{one}
	}
	var issued []ports.IceServer
	for _, s := range servers {
		urls := slices.DeleteFunc(slices.Clone(s.URLs), onPort53)
		if len(urls) > 0 {
			issued = append(issued, ports.IceServer{URLs: urls, Username: s.Username, Credential: s.Credential})
		}
	}
	return issued, nil
}

// onPort53 drops DNS-port relays: browsers block or stall on them.
func onPort53(url string) bool {
	host := url
	if i := strings.IndexByte(host, '?'); i >= 0 {
		host = host[:i]
	}
	return strings.HasSuffix(host, ":53")
}
