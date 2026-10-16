package ice

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

func TestProviders(t *testing.T) {
	ctx := t.Context()
	if got, err := (None{}).Issue(ctx, 1, time.Hour); err != nil || got != nil {
		t.Errorf("none = %v, %v", got, err)
	}
	s := Static{URLs: []string{"turn:turn.example:3478?transport=udp", "turns:turn.example:443"}, Username: "team", Credential: "secret"}
	got, err := s.Issue(ctx, 1, time.Hour)
	want := []ports.IceServer{{URLs: s.URLs, Username: "team", Credential: "secret"}}
	if err != nil || !cmp.Equal(got, want) {
		t.Errorf("static = %v, %v", got, err)
	}
	got[0].URLs[0] = "changed"
	if s.URLs[0] == "changed" {
		t.Error("issued servers share the configured slice")
	}
	if got, _ := (Static{}).Issue(ctx, 1, time.Hour); got != nil {
		t.Errorf("unconfigured static = %v", got)
	}
}

func TestCoturnREST(t *testing.T) {
	now := time.Unix(1_790_000_000, 0)
	c := CoturnREST{URLs: []string{"turn:coturn.example:3478"}, Secret: "north", Now: func() time.Time { return now }}
	got, err := c.Issue(t.Context(), 7, time.Hour)
	if err != nil || len(got) != 1 {
		t.Fatalf("issue = %v, %v", got, err)
	}
	if got[0].Username != "1790003600:7" {
		t.Errorf("username = %q, want expiry:user", got[0].Username)
	}
	mac := hmac.New(sha1.New, []byte("north"))
	mac.Write([]byte("1790003600:7"))
	if want := base64.StdEncoding.EncodeToString(mac.Sum(nil)); got[0].Credential != want {
		t.Errorf("credential = %q, want %q", got[0].Credential, want)
	}
	if got, _ := (CoturnREST{URLs: c.URLs, Now: c.Now}).Issue(t.Context(), 7, time.Hour); got != nil {
		t.Errorf("without a secret = %v", got)
	}
}

func TestCloudflare(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   []ports.IceServer
		err    bool
	}{
		{
			name:   "list, without DNS-port relays",
			status: http.StatusCreated,
			body: `{"iceServers":[{"urls":["stun:stun.cloudflare.com:3478","stun:stun.cloudflare.com:53"]},
				{"urls":["turn:turn.cloudflare.com:3478?transport=udp","turn:turn.cloudflare.com:53?transport=udp","turns:turn.cloudflare.com:443?transport=tcp"],"username":"u","credential":"c"}]}`,
			want: []ports.IceServer{
				{URLs: []string{"stun:stun.cloudflare.com:3478"}},
				{URLs: []string{"turn:turn.cloudflare.com:3478?transport=udp", "turns:turn.cloudflare.com:443?transport=tcp"}, Username: "u", Credential: "c"},
			},
		},
		{
			name:   "single object",
			status: http.StatusCreated,
			body:   `{"iceServers":{"urls":["turn:turn.cloudflare.com:3478"],"username":"u","credential":"c"}}`,
			want:   []ports.IceServer{{URLs: []string{"turn:turn.cloudflare.com:3478"}, Username: "u", Credential: "c"}},
		},
		{name: "rejected token", status: http.StatusUnauthorized, body: `{}`, err: true},
		{name: "garbage", status: http.StatusOK, body: `{"iceServers":"nope"}`, err: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct{ TTL int64 }
				_ = json.NewDecoder(r.Body).Decode(&req)
				if r.URL.Path != "/key-1/credentials/generate-ice-servers" || r.Header.Get("Authorization") != "Bearer tok" || req.TTL != 3600 {
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			c := Cloudflare{KeyID: "key-1", APIToken: "tok", Client: srv.Client(), BaseURL: srv.URL + "/"}
			got, err := c.Issue(t.Context(), 1, time.Hour)
			if (err != nil) != tt.err {
				t.Fatalf("err = %v, want error %v", err, tt.err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("servers (-want +got):\n%s", diff)
			}
		})
	}
}
