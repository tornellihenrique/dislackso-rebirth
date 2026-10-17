// Package ddns keeps a domain pointed at this machine's public address.
package ddns

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

// DuckDNSEndpoint is DuckDNS's update API.
const DuckDNSEndpoint = "https://www.duckdns.org/update"

// ErrRejected is DuckDNS refusing the update: the domain is not the token's.
var ErrRejected = errors.New("duckdns refused the update: check the domain and the token")

// DuckDNS updates a free <name>.duckdns.org.
type DuckDNS struct {
	Token    string
	Client   *http.Client
	Endpoint string
}

func (d DuckDNS) Point(ctx context.Context, domainName string, ip netip.Addr) error {
	name, ok := domain.DuckDNSName(domainName)
	if !ok {
		return fmt.Errorf("%q is not a duckdns.org domain", domainName)
	}
	q := url.Values{"domains": {name}, "token": {d.Token}}
	if ip.Is4() {
		q.Set("ip", ip.String())
	} else {
		q.Set("ipv6", ip.String())
	}
	endpoint := d.Endpoint
	if endpoint == "" {
		endpoint = DuckDNSEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+q.Encode(), http.NoBody)
	if err != nil {
		return fmt.Errorf("duckdns request: %w", withoutURL(err))
	}
	res, err := d.Client.Do(req)
	if err != nil {
		return fmt.Errorf("reach duckdns: %w", withoutURL(err))
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, 64))
	if err != nil {
		return fmt.Errorf("read duckdns answer: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("duckdns answered %d", res.StatusCode)
	}
	if strings.TrimSpace(string(body)) != "OK" {
		return ErrRejected
	}
	return nil
}

// withoutURL drops the URL an error quotes, and with it the token.
func withoutURL(err error) error {
	if ue, ok := errors.AsType[*url.Error](err); ok {
		return ue.Err
	}
	return err
}
