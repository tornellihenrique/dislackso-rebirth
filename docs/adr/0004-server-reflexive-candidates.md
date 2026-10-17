# ADR 0004: Server-reflexive candidates instead of a pinned node IP

- Status: accepted
- Date: 2026-10-17

## Context

Ledger D8 has LiveKit run full ICE behind NAT with its outside address from STUN and a single UDP
mux port. Phase 5 gave LiveKit that address as `rtc.node_ip` (the hub's own STUN probe), because
LiveKit's `use_external_ip` lookup prefers IPv6 and, on a host with broken IPv6, takes longer than
the supervisor waits for it to become healthy.

Either way LiveKit advertises its **local** port (7882) at the public address. That is only right
when the router keeps the port's number outside, or when 7882 is forwarded. The first night of
real use showed the common case is neither: the host's router keeps one outside port per inside
port (endpoint-independent mapping) but picks the number itself, and it offers no UPnP, NAT-PMP or
PCP. Every caller from another network then failed to reach `201.33.67.67:7882` and fell back to
the Cloudflare TURN relay (LiveKit log: every remote pair `relay`, five callers, ~10 GB of quota in
one evening). The relayed paths also lost packets, so LiveKit cut those callers to the half-size,
≤ 30 fps layer.

LiveKit (mediatransportutil `rtcconfig`) gives its own ICE agent the configured STUN servers only
when `node_ip` is empty (or auto-detected) and `use_external_ip` is off. Pion then gathers a
server-reflexive candidate per connection on a socket of its own, carrying the outside port the
router really assigned. With the e2e hub (no `node_ip`) on the same machine LiveKit advertised
`udp4 srflx 201.33.67.67:18472` next to its host candidates.

## Decision

- The hub never turns on LiveKit's `use_external_ip`.
- It writes `node_ip` only when its probe of UDP 7882 shows the port keeps its number outside
  (port-preserving NAT) or there is no NAT. Then 7882 at the public address is reachable and the
  single mux port is enough.
- Otherwise `node_ip` stays empty, so LiveKit gathers server-reflexive candidates through the STUN
  servers in `livekit.yaml`. Those candidates use ephemeral UDP ports next to the mux port.
- The connectivity self-test rates an endpoint-independent router **Good** instead of
  **Relay-dependent**.

## Consequences

- Callers whose own router is not strict connect directly to a host behind an ordinary home router
  without any router configuration; only strict (symmetric) NATs on either side still need the
  relay. A forwarded or port-preserving 7882 keeps working as before.
- Media may use ports other than 7882. The Windows firewall rule is per program
  (`livekit-server.exe`), so it covers them; a router mapping still targets 7882 only, which is
  why a host whose probe shows a symmetric NAT keeps relying on that mapping or the relay.
- Starting offline no longer needs special handling: LiveKit does no lookup of its own at start.
