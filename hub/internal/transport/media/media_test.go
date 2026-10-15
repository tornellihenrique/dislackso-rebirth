package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/coder/websocket"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

type parser struct {
	ev  ports.MediaEvent
	ok  bool
	err error
}

func (p parser) ParseWebhook(http.ResponseWriter, *http.Request) (ports.MediaEvent, bool, error) {
	return p.ev, p.ok, p.err
}

type handler struct{ got []ports.MediaEvent }

func (h *handler) HandleMediaEvent(_ context.Context, ev ports.MediaEvent) { h.got = append(h.got, ev) }

func TestProxy(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rtc/v1" && r.Header.Get("Upgrade") != "" {
			c, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			_ = c.Write(r.Context(), websocket.MessageText, []byte(r.URL.Query().Get("access_token")))
			_ = c.Close(websocket.StatusNormalClosure, "")
			return
		}
		_, _ = io.WriteString(w, r.Method+" "+r.URL.RequestURI())
	}))
	defer backend.Close()
	target, _ := url.Parse(backend.URL)
	var logs bytes.Buffer
	mux := http.NewServeMux()
	Register(mux, Options{Target: target, Webhooks: parser{}, Events: &handler{}, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	hub := httptest.NewServer(mux)
	defer hub.Close()

	get := func(path string) (int, string) {
		res, err := hub.Client().Get(hub.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = res.Body.Close() }()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(body)
	}
	if code, body := get("/lk/rtc/v1/validate?access_token=abc"); code != http.StatusOK || body != "GET /rtc/v1/validate?access_token=abc" {
		t.Errorf("validate: %d %q", code, body)
	}
	for _, path := range []string{"/lk/twirp/livekit.RoomService/ListRooms", "/lk/debug/rooms", "/lk/"} {
		if code, _ := get(path); code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, code)
		}
	}

	ws := "ws" + strings.TrimPrefix(hub.URL, "http") + "/lk/rtc/v1?access_token=tok"
	c, res, err := websocket.Dial(t.Context(), ws, nil)
	if res != nil && res.Body != nil {
		_ = res.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	_, msg, err := c.Read(t.Context())
	if err != nil || string(msg) != "tok" {
		t.Errorf("websocket: %q %v", msg, err)
	}
	_ = c.CloseNow()

	backend.Close()
	if code, _ := get("/lk/rtc/v1/validate?access_token=secret-token"); code != http.StatusBadGateway {
		t.Errorf("backend down: %d", code)
	}
	if strings.Contains(logs.String(), "secret-token") {
		t.Errorf("the log has the token: %s", logs.String())
	}
}

func TestWebhook(t *testing.T) {
	ev := ports.MediaEvent{Kind: ports.MediaParticipantJoined, Room: "vc_1"}
	post := func(p parser, remote string) (int, *handler) {
		h := &handler{}
		mux := http.NewServeMux()
		Register(mux, Options{Target: &url.URL{}, Webhooks: p, Events: h, Logger: slog.New(slog.DiscardHandler)})
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, WebhookPath, strings.NewReader("{}"))
		r.RemoteAddr = remote
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w.Code, h
	}
	if code, h := post(parser{ev: ev, ok: true}, "127.0.0.1:5000"); code != http.StatusNoContent || len(h.got) != 1 {
		t.Errorf("signed: %d %v", code, h.got)
	}
	if code, h := post(parser{ok: false}, "[::1]:5000"); code != http.StatusNoContent || len(h.got) != 0 {
		t.Errorf("ignored event: %d %v", code, h.got)
	}
	if code, h := post(parser{err: errors.New("bad signature")}, "127.0.0.1:5000"); code != http.StatusUnauthorized || len(h.got) != 0 {
		t.Errorf("bad signature: %d", code)
	}
	if code, h := post(parser{ev: ev, ok: true}, "203.0.113.7:5000"); code != http.StatusNotFound || len(h.got) != 0 {
		t.Errorf("from outside: %d", code)
	}
}
