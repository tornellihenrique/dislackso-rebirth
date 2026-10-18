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
	q := url.Values{}
	if ip.Is4() {
		q.Set("ip", ip.String())
	} else {
		q.Set("ipv6", ip.String())
	}
	return d.update(ctx, domainName, q)
}

// SetTXT sets the domain's TXT record, which DuckDNS also answers for every
// name under it, _acme-challenge included.
func (d DuckDNS) SetTXT(ctx context.Context, domainName, value string) error {
	return d.update(ctx, domainName, url.Values{"txt": {value}})
}

func (d DuckDNS) ClearTXT(ctx context.Context, domainName string) error {
	return d.update(ctx, domainName, url.Values{"txt": {"cleared"}, "clear": {"true"}})
}

func (d DuckDNS) update(ctx context.Context, domainName string, q url.Values) error {
	name, ok := domain.DuckDNSName(domainName)
	if !ok {
		return fmt.Errorf("%q is not a duckdns.org domain", domainName)
	}
	q.Set("domains", name)
	q.Set("token", d.Token)
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
