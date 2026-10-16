// Package media connects the hub's HTTP surface to the media server: a
// reverse proxy for client signaling under /lk, so it shares the instance's
// single public address, and the endpoint for the media server's webhooks.
// Media itself never passes through here.
package media

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/ingress"
)

const (
	ProxyPrefix = "/lk"
	WebhookPath = "/internal/livekit/webhook"
	// signalPrefix is the only part of the media server clients need; its
	// admin API and debug pages stay unreachable from outside.
	signalPrefix = "/rtc"
)

type WebhookParser interface {
	ParseWebhook(w http.ResponseWriter, r *http.Request) (ports.MediaEvent, bool, error)
}

type EventHandler interface {
	HandleMediaEvent(ctx context.Context, ev ports.MediaEvent)
}

type Options struct {
	// Target is the media server's HTTP address on localhost.
	Target   *url.URL
	Webhooks WebhookParser
	Events   EventHandler
	Logger   *slog.Logger
}

func Register(mux *http.ServeMux, o Options) {
	mux.Handle(ProxyPrefix+"/", proxy(o))
	mux.HandleFunc("POST "+WebhookPath, webhook(o))
}

func proxy(o Options) http.Handler {
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(o.Target)
			pr.Out.URL.Path = strings.TrimPrefix(pr.In.URL.Path, ProxyPrefix)
			pr.Out.URL.RawPath = ""
			pr.SetXForwarded()
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			// The request URL carries the participant's token in its query.
			if ue := (*url.Error)(nil); errors.As(err, &ue) {
				err = ue.Err
			}
			o.Logger.WarnContext(r.Context(), "media proxy", slog.String("path", r.URL.Path), slog.Any("error", err))
			http.Error(w, "media server unavailable", http.StatusBadGateway)
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(strings.TrimPrefix(r.URL.Path, ProxyPrefix), signalPrefix) {
			http.NotFound(w, r)
			return
		}
		rp.ServeHTTP(w, r)
	})
}

// webhook takes events from the media server on this machine. The signature
// is what proves the sender; the local check keeps the endpoint off every
// exposed address, tunnels included.
func webhook(o Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ingress.IsLocal(r) {
			http.NotFound(w, r)
			return
		}
		ev, ok, err := o.Webhooks.ParseWebhook(w, r)
		if err != nil {
			o.Logger.WarnContext(r.Context(), "media webhook refused", slog.Any("error", err))
			http.Error(w, "invalid webhook", http.StatusUnauthorized)
			return
		}
		if ok {
			o.Events.HandleMediaEvent(r.Context(), ev)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
