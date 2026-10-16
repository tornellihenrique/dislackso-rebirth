# ADR 0003: Ingresses, exposure settings and the port mapper

- Status: accepted
- Date: 2026-10-15

## Context

Phase 5 makes the hub reachable from other networks (ledger D7, D8). ARCHITECTURE §4.2 sketched an
`ExposureProvider` with `Start/Status/ClientIP/Stop`, CertMagic for `direct`, the exposure provider
in `config.yaml`, and `tailscale.com/net/portmapper` for router mappings. Building it surfaced
four problems:

- A tunnel's traffic reaches the hub from this machine (cloudflared connects over loopback). If
  tunnel and local requests shared one listener, every visitor would look local: setup and the
  LiveKit webhook trust local requests, and per-IP rate limits would collapse to one bucket.
- Switching providers is an admin action ("works without data loss" in the acceptance list), but
  `config.yaml` is read once at start and is not the admin panel's to write.
- LiveKit advertises its own UDP port with the address it learns over STUN. A router mapping to a
  different outside port is useless, and tailscale's portmapper asks UPnP routers for a random
  outside port.
- CertMagic brings a large dependency for one certificate the standard `x/crypto` module already
  obtains.

## Decision

- **One listener per ingress.** The hub's own `127.0.0.1` listener and each provider's listener
  serve the same handler, and each tags its requests with an `ingress.Info`: local or not, HTTPS
  or not (the cookie's `Secure` flag, HSTS), and how to find the client address (the TCP peer,
  Tailscale's funnel source, Cloudflare's `Cf-Connecting-Ip` on the tunnel's listener). Only the
  hub's own listener can be local, and no other ingress may report a loopback client.
- **Providers report, the service decides.** `ExposureProvider` is `Kind()` plus
  `Run(ctx, handler, report)`. The exposure service owns status, restarts and backoff, and makes a
  running provider's address the instance's public URL; status changes go to instance admins over
  the gateway.
- **Exposure, relay and host settings are instance settings in the DB**, their secrets in
  `secrets.yaml`, so the Connectivity page switches them at runtime.
- **Our own port mapper** (`adapter/portmap`) tries PCP, then NAT-PMP, then UPnP-IGD, always asking
  for the inside port outside and undoing any mapping on another port. The PCP nonce is kept
  across renewals.
- **`direct` uses `x/crypto/acme/autocert`** with the TLS-ALPN challenge on the listening port.
- **`cloudflared` is downloaded on first use** (pinned version, SHA-256) into the data folder
  instead of shipping in the archive, since most hosts use Tailscale.

## Consequences

- Ledger D7 and D8 hold as written; §4.2–4.4 and §5 describe the implementation.
- Each new provider supplies an `ingress.Info`; forgetting `ClientIP` falls back to the TCP peer,
  never to a forwarded header.
- Router support is limited to what the three protocols offer; hosts behind CGNAT or with every
  protocol off rely on the relay, which the self-test says.
