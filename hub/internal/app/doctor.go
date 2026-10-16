package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"text/tabwriter"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/clock"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/portmap"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/stun"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/doctor"
)

var natText = map[domain.NATType]string{
	domain.NATNoInternet:          "no internet connection",
	domain.NATOpen:                "none in the way (public address)",
	domain.NATPortPreserving:      "keeps ports, calls usually connect directly",
	domain.NATEndpointIndependent: "standard NAT",
	domain.NATSymmetric:           "strict NAT, calls need a relay",
}

var adviceText = map[string]string{
	domain.AdviceOffline:     "Connect this machine to the internet and restart the hub.",
	domain.AdviceEnableUPnP:  "Turn on UPnP or NAT-PMP in the router, or forward UDP 7882 and TCP 7881 to this machine.",
	domain.AdviceRelayNeeded: "People outside your network need a relay (TURN): set one up in Settings → Connectivity.",
	domain.AdviceRelayQuota:  "Calls from outside go through the relay and count against its monthly budget.",
}

// Doctor checks how reachable this machine is for calls and prints the
// result, for hosts without a browser (ARCHITECTURE §4.4).
func Doctor(ctx context.Context, out io.Writer) error {
	d := doctor.New(doctor.Deps{
		STUN: stunProbe, LocalAddrs: stun.LocalAddrs, Mapper: &portmap.Mapper{},
		Clock: clock.System{}, Logger: slog.New(slog.DiscardHandler),
	})
	t := d.Check(ctx)
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	row := func(label, value string) { _, _ = fmt.Fprintf(w, "%s\t%s\n", label, value) }
	row("Router:", natText[t.NAT])
	row("Public IPv4:", orNone(t.PublicIPv4))
	row("IPv6:", orNone(t.PublicIPv6))
	if t.Mapping.OK {
		row("Port forwarding:", "works ("+t.Mapping.Protocol+")")
	} else {
		row("Port forwarding:", "not available: "+t.Mapping.Detail)
	}
	row("Rating:", string(t.Rating))
	if err := w.Flush(); err != nil {
		return err
	}
	for _, a := range t.Advice {
		if _, err := fmt.Fprintln(out, "- "+adviceText[a]); err != nil {
			return err
		}
	}
	return nil
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
