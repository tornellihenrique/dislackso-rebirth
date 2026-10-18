package ddns

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

func TestDuckDNS(t *testing.T) {
	var got url.Values
	answer := "OK"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(answer))
	}))
	defer srv.Close()
	d := DuckDNS{Token: "secret-token", Client: srv.Client(), Endpoint: srv.URL}
	ctx := context.Background()

	if err := d.Point(ctx, "team.duckdns.org", netip.MustParseAddr("186.195.214.169")); err != nil {
		t.Fatal(err)
	}
	want := url.Values{"domains": {"team"}, "token": {"secret-token"}, "ip": {"186.195.214.169"}}
	if got.Encode() != want.Encode() {
		t.Errorf("query = %v, want %v", got, want)
	}

	answer = "KO"
	if err := d.Point(ctx, "team.duckdns.org", netip.MustParseAddr("186.195.214.169")); !errors.Is(err, ErrRejected) {
		t.Errorf("err = %v, want ErrRejected", err)
	}
	if err := d.Point(ctx, "team.example.com", netip.MustParseAddr("186.195.214.169")); err == nil {
		t.Error("pointed a domain that is not DuckDNS's")
	}
}

func TestDuckDNSTXT(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte("OK"))
	}))
	defer srv.Close()
	d := DuckDNS{Token: "secret-token", Client: srv.Client(), Endpoint: srv.URL}

	if err := d.SetTXT(t.Context(), "team.duckdns.org", "check-value"); err != nil {
		t.Fatal(err)
	}
	if want := (url.Values{"domains": {"team"}, "token": {"secret-token"}, "txt": {"check-value"}}); got.Encode() != want.Encode() {
		t.Errorf("set: query = %v, want %v", got, want)
	}
	if err := d.ClearTXT(t.Context(), "team.duckdns.org"); err != nil {
		t.Fatal(err)
	}
	if got.Get("clear") != "true" || got.Get("domains") != "team" {
		t.Errorf("clear: query = %v", got)
	}
}

func TestDuckDNSKeepsTheTokenOutOfErrors(t *testing.T) {
	d := DuckDNS{Token: "secret-token", Client: http.DefaultClient, Endpoint: "http://127.0.0.1:1/update"}
	err := d.Point(context.Background(), "team.duckdns.org", netip.MustParseAddr("192.0.2.1"))
	if err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Errorf("err = %v", err)
	}
}
