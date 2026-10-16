// Package publicprobe checks that an address works from the internet: the
// name through public DNS-over-HTTPS, since this machine's resolver may still
// hold an earlier "no such name", then an HTTPS request to the address found.
package publicprobe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Resolvers answer DNS-over-HTTPS in the JSON format Google and Cloudflare share.
var Resolvers = []string{"https://dns.google/resolve", "https://cloudflare-dns.com/dns-query"}

// ErrNotPublished means public DNS does not know the name yet.
var ErrNotPublished = errors.New("name not published yet")

const (
	requestTimeout = 20 * time.Second
	maxAnswer      = 64 << 10
	dnsNoSuchName  = 3
	dnsTypeA       = 1
	dnsTypeAAAA    = 28
)

type Prober struct {
	// Resolvers overrides the package's, for tests.
	Resolvers []string
	// RootCAs overrides the system's, for tests.
	RootCAs *x509.CertPool
	Client  *http.Client
}

func (p Prober) Reachable(ctx context.Context, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme != "https" {
		return fmt.Errorf("probe %s: not https", rawURL)
	}
	ips, err := p.resolve(ctx, u.Hostname())
	if err != nil {
		return err
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	var last error
	for _, ip := range ips {
		if last = p.get(ctx, u, net.JoinHostPort(ip, port)); last == nil {
			return nil
		}
	}
	return last
}

// get requests u from addr, checking the certificate against u's host.
func (p Prober) get(ctx context.Context, u *url.URL, addr string) error {
	var d net.Dialer
	client := &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return d.DialContext(ctx, network, addr)
			},
			TLSClientConfig:   &tls.Config{RootCAs: p.RootCAs, MinVersion: tls.VersionTLS12},
			DisableKeepAlives: true,
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("probe %s: %w", u.Host, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("probe %s: %s", u.Host, resp.Status)
	}
	return nil
}

type dohAnswer struct {
	Status int `json:"Status"`
	Answer []struct {
		Type int    `json:"type"`
		Data string `json:"data"`
	} `json:"Answer"`
}

// resolve asks each resolver in turn for the host's addresses.
func (p Prober) resolve(ctx context.Context, host string) ([]string, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []string{host}, nil
	}
	resolvers := p.Resolvers
	if len(resolvers) == 0 {
		resolvers = Resolvers
	}
	var last error
	for _, r := range resolvers {
		ips, err := p.lookup(ctx, r, host)
		if err == nil {
			return ips, nil
		}
		last = err
	}
	return nil, last
}

func (p Prober) lookup(ctx context.Context, resolver, host string) ([]string, error) {
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}
	var ips []string
	for _, typ := range []string{"A", "AAAA"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			resolver+"?name="+url.QueryEscape(host)+"&type="+typ, http.NoBody)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/dns-json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", host, err)
		}
		var ans dohAnswer
		err = json.NewDecoder(io.LimitReader(resp.Body, maxAnswer)).Decode(&ans)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", host, err)
		}
		if ans.Status == dnsNoSuchName {
			return nil, fmt.Errorf("resolve %s: %w", host, ErrNotPublished)
		}
		for _, a := range ans.Answer {
			if a.Type == dnsTypeA || a.Type == dnsTypeAAAA {
				ips = append(ips, a.Data)
			}
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("resolve %s: %w", host, ErrNotPublished)
	}
	return ips, nil
}
