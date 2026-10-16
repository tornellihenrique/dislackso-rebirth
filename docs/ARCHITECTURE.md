# Dislackso — Architecture & Product Specification

Status: **locked for v1 implementation**. Decisions in §2 are not to be revisited without the owner's approval.
Audience: Claude Code (implementer) and the team. Companion files: `docs/ROADMAP.md` (phases + acceptance criteria), `CLAUDE.md` (working rules).

---

## 0. Product summary

A self-hosted, Discord-like app for a small dev team, focused on **high-quality screen sharing with live Slack-style drawing**, voice channels and (later) text chat. Any member can host the whole backend on their own computer with **no static IP, no port forwarding and no paid backend**. Clients: browser (primary) and a thin desktop shell (Windows/macOS/Linux).

### Assumptions (design targets)
| Item | Target |
|---|---|
| Accounts per instance | ≤ 50 (designed for ~5–20 active) |
| Participants per voice channel | ≤ 12 typical, hard cap configurable (default 25) |
| Simultaneous screen shares per channel | ≤ 4 typical, cap configurable (default 6) |
| Max share quality | 2560×1440 @ 60 fps (instance cap, configurable lower) |
| Host machine | A team member's Windows/macOS/Linux PC on home/office broadband |
| Budget | US$0 recurring. Free tiers allowed only where the design degrades gracefully if they disappear |

### Non-goals (v1)
Federation between instances · mobile native apps · E2E encryption · recording · bots/slash commands · per-server role systems beyond owner/admin/member · remote control of shared screens.

### Glossary
| Term | Meaning |
|---|---|
| **Instance** | One deployment of the backend (one database, one set of accounts). Identified by its public URL. |
| **Hub** | The backend executable a host runs (Go). Owns the instance. |
| **Guild** | Code term for what the UI calls a **Server** (Discord-style group). Used in code to avoid clashing with "HTTP server". |
| **Channel** | Inside a guild: `category`, `voice`, `text`, `thread` (later `dm`). |
| **Room** | The LiveKit room backing exactly one voice channel (`vc_<channelId>`). |
| **Share** | A participant's published screen-share video track (+ optional screen audio). |
| **Stroke** | One drawing gesture on a share. |

---

## 1. Lessons from DiSlackso v1 (reference repo, do not copy its architecture)

| v1 problem | Root cause | v2 answer |
|---|---|---|
| Unstable calls | Hand-rolled P2P mesh with perfect-negotiation edge cases, renegotiation storms, per-viewer encoders | LiveKit SFU + official client SDK (built-in resume/reconnect, simulcast/SVC, dynacast, adaptive stream) |
| Sharer upload/CPU explodes with viewers | Mesh: one encode + one upload per viewer | SFU: one upload per sharer; viewers pick layers |
| Data loss risk | Whole DB as one JSON file mirrored into a single Supabase row; ephemeral Render disk | SQLite (WAL) with migrations, atomic transactions, nightly snapshots, optional continuous replication |
| Weird hosting | Cloud free tiers (Render cold starts, Supabase), desktop app bundled UI | Single hub binary on a member's PC, auto-exposed via tunnel; web UI served by the hub |
| "Frozen protocol" | Desktop app shipped its own UI → old clients pinned event names | Desktop shell loads the UI from the instance → server and UI always ship together |
| Images bloating the DB/socket payloads | Base64 images stored in the DB | Content-addressed blob store on disk (or S3-compatible) |
| No email / reset | Avoided SMTP | SMTP mailer (Gmail app password works free) + no-SMTP fallback |
| Untyped server, drift between client and server | CommonJS server, ad-hoc Socket.IO events | Protobuf contract → generated Go + TS; Connect RPC + typed gateway |
| Public best-effort TURN | Open Relay | Short-lived minted credentials from a real TURN provider (Cloudflare free tier), pluggable |
| No tests | Only `tsc` | Unit + integration (real LiveKit) + Playwright E2E with fake media |

**Keep from v1 (proven ideas):** strokes normalized to the *video content rect* (not the element); fade-out lifespan; author name tag at the pen tip; receiver-side anti-grief limits; desktop transparent overlay so the sharer sees drawings on their real screen; `contentHint` + explicit max bitrate/framerate + `maintain-resolution` for text; PTT released on window blur; speaking-indicator hangover (~400 ms); mute/deafen persisted outside calls; invite codes from an unambiguous alphabet (no I/O/0/1); identical error for "unknown user" and "wrong password"; media engine kept outside React; a single gateway listener feeding stores; `<video>` attached via callback ref; fullscreen tile through a portal.

---

## 2. Decision ledger (locked)

| # | Decision | One-line rationale |
|---|---|---|
| D1 | **Media = LiveKit SFU**, self-hosted as a sidecar managed by the hub. No mesh. | Discord-class quality and stability with one upload per sharer and adaptive layers. |
| D2 | **Hub in Go**, single executable per OS, supervises sidecars (`livekit-server`, optional `cloudflared`). | One-file install, low memory, robust process control, native LiveKit/Tailscale libraries. |
| D3 | **SQLite (WAL)** via pure-Go driver (`modernc.org/sqlite`), `sqlc` queries, `goose` migrations. Nightly `VACUUM INTO` snapshots. Optional Litestream → S3-compatible. | Zero-ops, transactional, cross-compiles without CGO. |
| D4 | **Protobuf contract** (Buf): Connect RPC for request/response, protobuf frames for the gateway and drawing packets. Generated Go + TS are committed; CI checks they are fresh. | One source of truth for every wire format. |
| D5 | **Own WebSocket gateway** (Discord-style Hello → Identify/Resume → Ready, heartbeats, sequence replay). | Deterministic reconnection; one place where push events enter the client. |
| D6 | **Hub is the authority for voice state.** LiveKit webhooks reconcile. One voice connection per user (LiveKit identity = userId). | Consistent sidebar state across all guilds; "moved to another device" semantics. |
| D7 | **Exposure is pluggable.** Default: **Tailscale Funnel via embedded `tsnet`** (free, stable `https://<name>.<tailnet>.ts.net`, no domain, no separate install). Alternatives: Cloudflare named tunnel (own domain), Cloudflare quick tunnel (trial), direct (public IP/VPS), LAN-only. | Reachable HTTPS/WSS without port forwarding or budget. |
| D8 | **Media reachability ladder:** LiveKit full-ICE behind NAT (external IP via STUN, single UDP mux port, IPv6, internal IP for LAN) + automatic UPnP/NAT-PMP/PCP mapping + **client TURN fallback with short-lived credentials minted by the hub** (default provider: Cloudflare Realtime TURN, 1 TB/month free) + built-in connectivity self-test and TURN budget guard. | Works behind most NATs/CGNAT with zero router config; degrades to relay instead of failing. |
| D9 | **Web client is embedded in and served by the hub** (same origin, HttpOnly cookie auth). | No CORS, no version skew, XSS-resistant sessions. |
| D10 | **Desktop = thin Electron shell** that loads the instance URL and exposes a small, versioned native bridge (screen picker, system audio, drawing overlay, global hotkeys, tray, notifications, deep links, host mode). | Consistent Chromium WebRTC; native features without shipping a second UI. |
| D11 | **Drawing transport = LiveKit data packets** in the voice room (lossy point batches + reliable begin/end/clear), protobuf-encoded; coordinates normalized to the share's video frame. Shared `@dislackso/annotation` package renders on web tiles and the desktop overlay. | Low latency, automatically scoped to the channel, no hub fan-out. |
| D12 | **Accounts:** username + email + password; argon2id; opaque session tokens stored hashed; email verification + password reset via SMTP; no-SMTP mode supported. | Minimal info, standard security. |
| D13 | **Guild roles:** `owner` / `admin` / `member`, all checks go through one `Policy` component. | Simple now, extensible to roles later without touching services. |
| D14 | **IDs:** 64-bit Snowflake (timestamp-ordered) in DB/Go; `string` on the wire. | Sortable pagination, compact, JS-safe. |
| D15 | **UI stack:** React 19 + TypeScript (strict) + Vite + Tailwind v4 + Radix primitives + Lucide icons + TanStack Router/Query + Zustand. Discord-like layout, original branding. i18n from day one (`en`, `pt-BR`). | Fast to build, accessible, familiar UX. |
| D16 | **Registration default = invite-only** (a valid guild invite or instance invite is required to register). | Instance is publicly reachable; prevents drive-by signups. |
| D17 | **Repo = monorepo**: `hub/` (Go), `apps/web`, `apps/desktop`, `packages/*` (pnpm workspaces), `proto/`. Task runner: `Taskfile.yml` (cross-platform). | One PR can change contract + server + client atomically. |

---

## 3. System overview

```
                        ┌──────────────────────────── Host PC ─────────────────────────────┐
 Browser / Desktop      │                                                                   │
 shell (Electron)       │   ┌──────────────────── hub (Go, single binary) ───────────────┐  │
 ───────────────        │   │  HTTP mux                                                   │  │
  HTTPS + WSS  ─────────┼──►│   /            web client (embedded SPA)                    │  │
  via exposure          │   │   /rpc/*       Connect RPC (protobuf)                       │  │
  (tsnet Funnel /       │   │   /gateway     realtime WebSocket                           │  │
   cloudflared /        │   │   /lk/*        reverse proxy → LiveKit signaling            │  │
   direct)              │   │   /files/*     blob downloads   /api/uploads  multipart     │  │
                        │   │   /internal/livekit/webhook  (localhost only)               │  │
                        │   │  services · policy · gateway hub · sqlite · blob store      │  │
                        │   │  sidecar supervisor · exposure · portmapper · ICE provider  │  │
                        │   └───────────────┬─────────────────────────────────────────────┘  │
                        │                   │ Twirp API + webhooks (127.0.0.1)              │
                        │   ┌───────────────▼──────────────┐                               │
  WebRTC media  ◄───────┼──►│ livekit-server (sidecar)     │  UDP 7882 mux (+TCP 7881)     │
  (UDP direct / IPv6 /  │   │ full ICE, ext IP via STUN    │  auto-mapped via UPnP/PCP     │
   via TURN relay)      │   └──────────────────────────────┘                               │
                        └───────────────────────────────────────────────────────────────────┘
        ▲                                   ▲
        │ TURN (UDP/TCP/TLS 443)            │ STUN / relayed media
        └────────── Cloudflare Realtime TURN (credentials minted by hub per join) ─────────┘
```

Media never passes through the HTTP exposure path. Only signaling, API, chat and files do.

---

## 4. Hosting & connectivity

### 4.1 Target host experience (linear)
1. Download the hub release for the OS (zip contains `hub` + `livekit-server`), or click **Host an instance** in the desktop shell.
2. Run it. The hub opens `http://localhost:7300/setup` (setup is only reachable from localhost and requires the one-time setup token printed in the console/log).
3. Wizard: instance name → create instance-admin account → optional SMTP → finish. Setup then continues on **Settings → Connectivity**: choose exposure (Tailscale Funnel recommended: click the login link, approve, done) → the connectivity self-test result → optional TURN credentials and a browser relay test.
4. The Connectivity page shows the public URL; Settings → Instance creates invite links. Copy, send to the team. `docs/HOSTING.md` is the host's step-by-step guide.
5. Optional toggles on the Connectivity page: run at login, keep PC awake while hosting; auto-update later.

CLI equivalents for headless hosts: `hub run`, `hub setup`, `hub doctor`, `hub backup`, `hub user reset-link <username>`, `hub version`.

### 4.2 Exposure providers (`ExposureProvider` interface)
```go
type ExposureProvider interface {
    Kind() domain.ExposureKind
    // Run serves handler until ctx ends, reporting every state change
    // (starting, needs_login + login URL, running + public URL, failed + problem code).
    Run(ctx context.Context, handler http.Handler, report func(domain.ExposureStatus)) error
}
```
Every way into the hub is its own listener tagged with an **ingress** (`transport/ingress`): whether it is the hub's own `127.0.0.1` listener (the only one that can be "local": setup, LiveKit webhooks), whether the client used HTTPS (cookie `Secure`, HSTS), and whom to trust for the client IP (the TCP peer, Tailscale's funnel source, Cloudflare's `Cf-Connecting-Ip` on the tunnel's listener). Exposure settings are instance settings in the DB, switched from the admin panel; the Cloudflare tunnel token lives in `secrets.yaml`. The exposure service runs the configured provider, restarts it when settings change, retries failures (5 s to 60 s backoff), makes a running provider's URL the instance's public URL, and pushes each status change to instance admins. While starting, a provider reports a stage (`certificate`, `publishing`); Tailscale Funnel reports running only after its address answers from the internet (`PublicProbe`: public DNS-over-HTTPS, then HTTPS to the address found), or after 15 minutes as `unverified`.
| Provider | How | URL | Notes |
|---|---|---|---|
| `tailscale-funnel` (**default**) | Embedded `tailscale.com/tsnet`, `ListenFunnel("tcp", ":443")` | `https://<hostname>.<tailnet>.ts.net` stable | Host needs a free Tailscale account; first run shows login URL; surface "enable Funnel/HTTPS" guidance from errors. Bandwidth is limited by Tailscale (unspecified) — fine for API/chat, slower for large files. |
| `cloudflare-tunnel` | `cloudflared tunnel run` sidecar with the token in its environment (downloaded on demand, pinned version + SHA-256); the public hostname routes to `http://localhost:7301`, a listener of its own | Team's domain, stable | Best throughput; prerequisite for multi-host failover (§17.7). Needs a domain on Cloudflare (~US$10/yr). |
| `cloudflare-quick` | `cloudflared tunnel --url http://127.0.0.1:<own port>` | Random `*.trycloudflare.com`, changes every start | Trials only: 200 in-flight request cap, no SLA. Hub shows the new URL loudly. |
| `direct` | Hub listens on `:443` with a Let's Encrypt certificate (`x/crypto/acme/autocert`, TLS-ALPN challenge on the same port) | Own | For VPS/public IP/port-forward users. |
| `lan` | Bind on LAN interface with a self-signed certificate for the machine's private addresses | `https://<lan-ip>:7443` | Office-only use. `getUserMedia` needs HTTPS outside localhost. |

`/lk/*` is reverse-proxied to `127.0.0.1:7880` (WebSocket upgrade supported) so LiveKit signaling shares the single public hostname. Client connects LiveKit with `url = wss://<instance>/lk`.

### 4.3 Media reachability ladder
The hub generates `livekit.yaml` (never hand-edited) with:
```yaml
port: 7880                      # bind_addresses: [127.0.0.1]
rtc:
  udp_port: 7882                # single UDP mux port (simplifies mapping)
  tcp_port: 7881                # useful only when mapped
  use_external_ip: true
  skip_external_ip_validation: true   # NAT without hairpin
  advertise_internal_ip: true         # LAN peers connect directly
  stun_servers: [stun.cloudflare.com:3478, stun.l.google.com:19302]
  # use_ice_lite stays false (full ICE is required behind NAT)
room: { auto_create: true, empty_timeout: 30, departure_timeout: 20 }
keys: { <generated>: <generated> }
webhook: { api_key: <generated>, urls: [http://127.0.0.1:7300/internal/livekit/webhook] }
logging: { level: info }
```
The API key pair is generated once and kept in `secrets.yaml`; the sidecar receives it through `LIVEKIT_KEYS`, so `livekit.yaml` holds no secret. LiveKit rejects unknown keys and refuses to start when `use_external_ip` cannot reach STUN, so hosts without internet access set `livekit.external_ip: false` in the hub config.

Connection paths, in order of preference:
1. **LAN host candidate** (same office network).
2. **IPv6** global address (common on Brazilian fiber; stateful firewalls are opened by full-ICE checks from both sides).
3. **IPv4 direct** — works when the host NAT preserves the mapped port, or after the hub creates a mapping for UDP 7882 (+TCP 7881) with PCP, NAT-PMP or UPnP-IGD (`adapter/portmap`, tried in that order). The outside port must equal the inside one, since LiveKit advertises its own port with the STUN-discovered address, so a router offering another port is refused (tailscale's portmapper picks other ports, hence our own). Mapping is renewed and removed on shutdown.
4. **TURN relay** — clients receive short-lived TURN credentials from the hub at voice join and pass them as `rtcConfig.iceServers` to `Room.connect` (verify livekit-client merge/override semantics). Because LiveKit runs full ICE, it sends checks to the client's relay candidate, which opens the host NAT toward the relay; works even for symmetric host NAT.

`IceServerProvider` interface:
```go
type IceServerProvider interface {
    Name() string
    Issue(ctx context.Context, userID domain.ID, ttl time.Duration) ([]IceServer, error)
}
```
Implementations: `cloudflare` (POST `https://rtc.live.cloudflare.com/v1/turn/keys/{keyId}/credentials/generate-ice-servers` with `{"ttl": N}`; drop port-53 URLs), `static` (any provider with fixed username/password, e.g. ExpressTURN free 1 TB/mo), `coturn-rest` (shared-secret REST credentials for a self-hosted coturn), `none` (STUN only).

### 4.4 Connectivity self-test (`hub doctor` + admin panel)
Runs at every start (background, after the first forwarding attempt and once LiveKit is up; only when `livekit.external_ip` is on) and on demand from the admin panel; the latest result is kept in memory and shown with a **Host suitability** rating. `hub doctor` runs the network checks from a console, trying port forwarding on a throwaway port so a running hub's mappings stay untouched:
1. LiveKit up (Twirp `ListRooms`).
2. Before LiveKit binds 7882: bind UDP 7882, query two STUN servers → public IPv4, mapped ports → classify *port-preserving*, *endpoint-independent*, *endpoint-dependent (symmetric)*.
3. IPv6: global address present? STUN over udp6.
4. Port mapping attempt (UPnP/NAT-PMP/PCP) → success + external address.
5. Relay test (browser, from the admin panel): `livekit-client` joins a diagnostics room with `iceTransportPolicy: 'relay'` through the configured TURN → proves remote relayed clients work.
6. Rating: **Excellent** (mapping OK or IPv6), **Good** (port-preserving NAT), **Relay-dependent** (symmetric/CGNAT, no IPv6) with actionable advice ("enable UPnP on router", "this host will consume TURN quota", "consider another host").

Runtime telemetry: each client reports its selected ICE candidate-pair type and bytes every 30 s over the gateway. Admin panel shows % relayed and an **estimated monthly TURN usage**. **TURN budget guard:** `turn.monthly_budget_gb` (default 900). Above 90 % the hub warns admins (log and admin panel); above 100 % it stops issuing TURN credentials (STUN only) unless an admin overrides. Budgets count decimal gigabytes, as providers bill them.

### 4.5 Host bandwidth guide (shown in admin panel)
SFU egress ≈ Σ (bitrate each viewer receives). Screen content averages far below caps when mostly static.

| Scenario | Host upload (peak) | Host download |
|---|---|---|
| 6 people, 1 share 1080p60 (cap 6 Mbps) | ~32 Mbps | ~8 Mbps |
| 6 people, 2 shares 1440p60 (cap 10 Mbps), each viewer focuses one | ~60 Mbps | ~25 Mbps |
| Voice only, 10 people | ~5 Mbps | ~1 Mbps |

### 4.6 Host conveniences
- **Keep awake while hosting** (Windows `SetThreadExecutionState`, macOS `caffeinate`-equivalent IOKit assertion, Linux `systemd-inhibit`).
- **Run at login** (Windows `Run` registry key of the user; macOS LaunchAgent; Linux XDG autostart entry for desktop sessions), repeating the command line the hub was started with.
- **Firewall helper** (Windows): button that elevates once to add an inbound UDP rule for `livekit-server.exe`.
- **Self-update**: check GitHub Releases, download, verify SHA-256, swap binary, restart (Phase 11).
- **Alternative deployment** (documented, not default): run the hub on an always-free cloud VM with a public IP (`direct` exposure, no TURN needed).

---

## 5. Hub architecture (Go)

### 5.1 Layering (dependency rule: inward only)
```
transport (rpc, gateway, http)  →  service (use cases)  →  domain (entities, errors, policy)
                                         ↓ depends on interfaces (ports)
adapters (sqlite, smtp, livekit, ice, exposure/{tailscale,cloudflared,direct}, portmap, stun, hostos, blob) implement ports
app (composition root) wires everything from config — the only place that knows concrete types
```
- **domain**: entities, value objects, domain errors (`ErrNotFound`, `ErrForbidden`, `ErrConflict`, `ErrRateLimited`, `ErrInvalid`), `Policy` (pure functions over actor/guild/member).
- **ports**: small interfaces (ISP). `UserRepo`, `SessionRepo`, `GuildRepo`, `MemberRepo`, `ChannelRepo`, `InviteRepo`, `BanRepo`, `MessageRepo`, `FileRepo`, `AuditRepo`, `SettingsRepo`, `TxRunner`, `Clock`, `IDGen`, `PasswordHasher`, `TokenGen`, `Mailer`, `EventPublisher`, `MediaProvider`, `IceServerProvider`, `BlobStore`, `ExposureProvider`, `RateLimiter`.
- **service**: one package per bounded area — `auth`, `account`, `guild`, `channel`, `invite`, `moderation`, `voice`, `presence`, `message`, `file`, `social`, `instance`. Services never import transport or adapters.
- **transport/rpc**: thin Connect handlers: authenticate → map proto ↔ domain → call service → map errors to Connect codes. No business logic.
- **transport/gateway**: connection lifecycle, encoding, replay buffer, subscription topics; consumes `EventPublisher` events.
- **transport/http**: static SPA (with `index.html` fallback), uploads, file downloads, `/lk` proxy, webhooks, health.
- **events**: in-process typed pub/sub (`EventBus`) — interface allows a different bus later.
- **sidecar**: process supervisor (start, health probe, restart with backoff, log capture to `logs/<name>.log`, graceful stop; Windows Job Object with `KILL_ON_JOB_CLOSE` so children die with the hub; process groups on Unix).

### 5.2 `MediaProvider` port (LiveKit adapter)
```go
type MediaProvider interface {
    URL() string                                                             // where clients connect, e.g. "/lk"
    Available() bool                                                         // the media server is up
    JoinToken(j MediaJoin) (string, error)                                   // room, identity, metadata, permissions, ttl
    RemoveParticipant(ctx context.Context, room, identity string) error
    SetPermissions(ctx context.Context, room, identity string, p VoicePermissions) error // publish sources + subscribe
    ListRooms(ctx context.Context) ([]string, error)
    ListParticipants(ctx context.Context, room string) ([]MediaParticipant, error)
}
```
`SetPermissions` replaces the whole permission set; LiveKit unpublishes tracks whose source is no longer allowed, so server mute needs no separate track mute. The adapter also verifies and parses webhooks (`ParseWebhook`), which the HTTP handler reaches through an interface of its own.
`url` can point to the embedded sidecar (`wss://<instance>/lk`) or an **external LiveKit** (VPS or LiveKit Cloud) — configurable, same code path.

### 5.3 Configuration & data directory
Precedence: defaults < `config.yaml` < env (`DSX_*`) < flags. Only bootstrap settings live in the file (ports, data dir, media server). Everything else is runtime instance settings in the DB, editable in the admin panel, including exposure, relay (TURN) and email; their secrets live in `secrets.yaml`.

```
<data-dir>/                      Windows: %APPDATA%\Dislackso\hub   macOS: ~/Library/Application Support/Dislackso/hub   Linux: ~/.local/share/dislackso/hub
  config.yaml   secrets.yaml (0600)   data.db (+wal/shm)   files/   backups/   logs/   tsnet/   sidecars/ (cloudflared)   acme/   tls/   livekit.yaml (generated)
```

### 5.4 Security baseline
- Passwords: argon2id (m=64 MiB, t=3, p=2; parameters stored in hash string). Constant-time compare.
- Sessions: 32-byte random token; DB stores SHA-256; cookie `dsx_session` HttpOnly, Secure, SameSite=Lax, 30-day sliding expiry. Desktop uses the same cookie (same origin inside the shell partition).
- CSRF: cookie auth + Connect requests require `Connect-Protocol-Version`/`Content-Type: application/proto|json` (forces preflight cross-origin) + `Origin` check on state-changing routes and on gateway upgrade (prevents cross-site WebSocket hijacking).
- Rate limits (token bucket, per IP + per account): login, register, verify resend, reset request, invite accept, message send, uploads, gateway frames.
- Validation: rules live in `domain` validators and reach clients as typed `FieldViolations` error details. `protovalidate` annotations are deferred (see STATUS, Phase 1 decisions).
- Uploads: magic-byte sniffing, size caps, images re-encoded (metadata stripped), downloads with `X-Content-Type-Options: nosniff`, `Content-Security-Policy: sandbox`, `Content-Disposition: attachment` for non-media types.
- Headers: strict CSP for the SPA, `Referrer-Policy: same-origin`, HSTS when HTTPS. `style-src` admits a per-response nonce, handed to the page in `<meta name="csp-nonce">`, because UI libraries inject `<style>` elements (scroll locking in modal dialogs); scripts stay `'self'`, plus `'wasm-unsafe-eval'` so the noise suppressor can compile its WebAssembly (it allows no JavaScript eval).
- Link unfurling (chat phase): SSRF guard (deny private/loopback/link-local, re-check after DNS resolution, size/time limits).
- Client IP only from the ingress the request arrived on (§4.2); forwarded headers are trusted only on a tunnel's own listener, and only the hub's own listener can yield a loopback (local) client.
- Secrets never logged; logs rotate (lumberjack).

---

## 6. Data model (SQLite)

Conventions: `INTEGER` Snowflake PKs; timestamps as Unix ms `INTEGER`; soft delete via `deleted_at` where history matters; `STRICT` tables; foreign keys ON; WAL; `busy_timeout`; single writer connection + reader pool.

```
users(id PK, username TEXT UNIQUE COLLATE NOCASE, email TEXT UNIQUE COLLATE NOCASE, email_verified_at, password_hash,
      display_name, avatar_file_id, status_pref TEXT /*online|idle|dnd|invisible*/, custom_status,
      is_instance_admin INT, disabled_at, created_at, updated_at, deleted_at)
email_tokens(id PK, user_id, purpose TEXT /*verify|reset|change_email|instance_invite*/, token_hash UNIQUE, payload, expires_at, used_at, created_at)
sessions(id PK, user_id, token_hash UNIQUE, created_at, last_seen_at, expires_at, user_agent, ip, revoked_at)
guilds(id PK, name, icon_file_id, owner_id, created_at, updated_at, deleted_at)
guild_members(guild_id, user_id, role TEXT /*owner|admin|member*/, joined_at, PRIMARY KEY(guild_id,user_id))
guild_bans(guild_id, user_id, banned_by, reason, created_at, PRIMARY KEY(guild_id,user_id))
invites(code TEXT PK, guild_id, created_by, max_uses, uses, expires_at, revoked_at, created_at)
channels(id PK, guild_id NULL /*null for DMs*/, parent_id NULL, type TEXT /*category|voice|text|thread|dm*/, name, topic,
         position, user_limit, drawing_allowed INT, max_share_quality TEXT, last_message_id, archived_at, created_at, updated_at, deleted_at)
messages(id PK, channel_id, author_id, content, reply_to_id, thread_id, flags, edited_at, deleted_at)  -- created time = id timestamp
attachments(message_id, file_id, position, PRIMARY KEY(message_id,file_id))
files(id PK, owner_id, sha256, size, mime, original_name, width, height, duration_ms, thumb_file_id, storage_key, created_at)
reactions(message_id, user_id, emoji, created_at, PRIMARY KEY(message_id,user_id,emoji))
read_states(user_id, channel_id, last_read_id, mention_count, PRIMARY KEY(user_id,channel_id))
relationships(user_id, other_id, kind TEXT /*friend|pending_out|pending_in|blocked*/, created_at, PRIMARY KEY(user_id,other_id))
dm_participants(channel_id, user_id, closed_at, PRIMARY KEY(channel_id,user_id))
audit_log(id PK, guild_id, actor_id, action TEXT, target_type, target_id, data_json, created_at)
webhooks(id PK, channel_id, name, avatar_file_id, token_hash, created_by, created_at)   -- optional feature
instance_settings(key TEXT PK, value_json, updated_at)
```
- **Voice states and presence are in-memory only** (rebuilt from LiveKit `ListParticipants` on hub restart; clients rejoin automatically).
- FTS5 table `messages_fts(content)` for search (optional phase).
- Message retention: unlimited by default; admin-configurable per instance.
- Backups: nightly `VACUUM INTO backups/data-YYYYMMDD.db`, keep 7 (configurable); `files/` is content-addressed so incremental copy is trivial. Optional Litestream (§17.7).

Snowflake: 41-bit ms since custom epoch (2026-01-01) | 10-bit worker (0 by default) | 12-bit sequence.

---

## 7. API contract

### 7.1 Layout
```
proto/
  buf.yaml  buf.gen.yaml
  dislackso/v1/        common.proto, auth.proto, account.proto, guild.proto, channel.proto, invite.proto,
                       moderation.proto, voice.proto, message.proto, social.proto, instance.proto, file.proto
  dislackso/gateway/v1/gateway.proto
  dislackso/annotation/v1/annotation.proto
```
Generated: Go → `hub/gen/...` (connect-go, protobuf-go); TS → `packages/contracts/src/gen` (`@bufbuild/protobuf` v2, `@connectrpc/connect`). Committed. `task gen` regenerates; CI fails if output differs.

### 7.2 Conventions
- IDs are `string` (decimal Snowflake). Timestamps `int64` Unix ms.
- Errors → Connect codes: NotFound, PermissionDenied, AlreadyExists (conflicts), InvalidArgument (with field violations), ResourceExhausted (rate limit, with retry-after detail), Unauthenticated, FailedPrecondition (e.g. email not verified).
- Pagination: `before`/`after` id + `limit` (max 100).
- Mutations return the updated entity; the same change is also broadcast on the gateway (clients dedupe by entity version/updatedAt).
- Large binaries never go through RPC: `POST /api/uploads` (multipart, streamed to blob store, returns `FileRef`), downloads via `GET /files/{id}/{name}` (auth by session cookie; thumbnails `?size=`).

### 7.3 Services (v1 scope; chat/social marked *opt*)
- `AuthService`: Register, VerifyEmail, ResendVerification, Login, Logout, RequestPasswordReset, ResetPassword, ConfirmEmailChange, GetSetupState.
- `AccountService`: GetMe, UpdateProfile(display_name, avatar_file_id), ChangePassword, RequestEmailChange, ListSessions, RevokeSession, RevokeOtherSessions, DeleteAccount, SetStatusPreference.
- `GuildService`: Create, Update, Delete, Leave, TransferOwnership, SetMemberRole, ListMembers.
- `ChannelService`: Create, Update, Delete, Reorder.
- `InviteService`: Create, List, Revoke, Preview(code), Accept(code).
- `ModerationService`: Kick, Ban, Unban, ListBans, ListAuditLog, DeleteMessage (any).
- `VoiceService`: Join(channel_id) → {livekit_url, token, ice_servers, voice_session_id, limits}, Leave, Move(user, channel), SetServerMute, SetServerDeafen, Disconnect(user).
- `InstanceService` (instance admin): GetSettings, UpdateSettings, ListUsers, SetUserDisabled, SetUserInstanceAdmin, CreatePasswordResetLink, CreateInstanceInvite, SendTestEmail.
- `ConnectivityService` (instance admin): GetConnectivity, SetExposure, SetTurn, RunSelfTest, StartRelayTest, SetHostOptions and AddFirewallRule (both only from the hosting machine).
- *opt* `MessageService`: List, Send, Edit, Delete, AddReaction, RemoveReaction, Ack (read state), Search, ListPins, Pin/Unpin.
- *opt* `ThreadService`: Create(from message or blank), List(active/archived), Archive.
- *opt* `SocialService`: SendFriendRequest, Accept, Remove, Block, Unblock, ListRelationships, OpenDM, CloseDM.

---

## 8. Realtime gateway

### 8.1 Lifecycle
```
client → WS /gateway (cookie auth, Origin checked)
server → Hello{heartbeat_interval_ms: 25000, session_resume_window_ms: 120000}
client → Identify{client_info, capabilities}            | Resume{session_id, last_seq}
server → Ready{session_id, snapshot}                     | Resumed (then replays missed events)  | InvalidSession{resumable:false}
client ↔ Heartbeat{last_seq} / HeartbeatAck   (server closes after 2 missed intervals)
server → Dispatch{seq, event}  (seq monotonic per session)
```
- Per-session replay ring buffer: 2,000 events or 120 s. Resume beyond → InvalidSession → client re-identifies and rebuilds from Ready.
- Client reconnect: exponential backoff 0.5 s → 15 s with jitter; try Resume first. `navigator.onLine`/visibility events trigger immediate retry.
- Close codes: 4000 unknown, 4001 bad opcode, 4002 decode error, 4003 not authenticated, 4004 session revoked, 4005 already identified, 4007 invalid seq, 4008 rate limited, 4009 session expired, 4010 hub restarting (reconnect later, no resume).
- Frames are binary protobuf (`ClientFrame`/`ServerFrame`). Dev builds expose a frame inspector panel.
- Services publish events only after their transaction commits, and Identify subscribes to the user's topics before reading the snapshot; events that arrive in between are sent right after Ready. Clients upsert by `updated_at`, so a duplicate is harmless.
- Each heartbeat re-checks the session cookie: a signed-out, revoked, expired or disabled session ends the connection with 4004 within one heartbeat interval.
- A connection that drops without a close frame keeps its session (and the user's presence) for the resume window; a normal close (tab closed, sign-out) ends both at once.

### 8.2 Ready snapshot
`me`, `settings` (synced user prefs), all guilds the user belongs to with channels, members (+ public user profiles), presences, **voice states for every guild**, read states, relationships (opt), open DMs (opt), instance info (name, limits, features). Sized for ≤ 50 users; no lazy member loading needed.

### 8.3 Fan-out topics
`guild:<id>` (all members), `user:<id>` (all sessions of a user), `channel:<id>` (typing; text viewers), `admins:<guildId>`, `instance-admins`. Gateway subscribes sessions on Identify and updates subscriptions on membership changes.

### 8.4 Event catalog
`GUILD_CREATE` (joined/created) · `GUILD_UPDATE` · `GUILD_DELETE{reason: deleted|kicked|banned|left}` · `MEMBER_ADD/UPDATE/REMOVE` · `CHANNEL_CREATE/UPDATE/DELETE` · `CHANNELS_REORDERED` · `USER_UPDATE` (public profile of anyone sharing a guild/relationship) · `PRESENCE_UPDATE` · `VOICE_STATE_UPDATE` · `VOICE_FORCED{action: disconnected|moved|kicked, by}` · `INVITE_CREATE/DELETE` (admins) · `BAN_ADD/REMOVE` (admins) · `AUDIT_ENTRY` (admins) · `ME_UPDATE` (the receiver's own account, for their other sessions; it also moves the instance-admins subscription) · `EXPOSURE_CHANGED` (instance admins) · `INSTANCE_NOTICE` · *opt:* `MESSAGE_CREATE/UPDATE/DELETE`, `REACTION_ADD/REMOVE`, `TYPING_START`, `READ_STATE_UPDATE`, `THREAD_*`, `RELATIONSHIP_*`, `DM_CREATE`.

Client → server frames: `UpdatePresence{idle}` (inactivity; the status preference and custom status are set through `AccountService.SetStatusPreference`, so they persist and reach every device), `UpdateVoiceSelfState{self_mute, self_deaf}`, `TypingStart{channel_id}` (≤ 1 per 8 s), `ClientTelemetry{voice_session_id, candidate, bytes_sent, bytes_received, rtt_ms}` every 30 s while in a call (bytes summed over the call's peer connections, growing only).

### 8.5 Presence
Online while ≥ 1 gateway session; `idle` reported by client after 10 min without input (desktop uses `powerMonitor` idle time); `dnd` and `invisible` are user preferences (invisible = shown offline to others). Presence is broadcast to every guild the user is in.

---

## 9. Voice & media

### 9.1 Voice state (in-memory, hub-owned)
```
VoiceState{ user_id, guild_id, channel_id, voice_session_id, self_mute, self_deaf, server_mute, server_deaf,
            streaming /*from LiveKit track events*/, camera /*opt*/, connected /*false until LiveKit confirms*/, joined_at }
```
Broadcast via `VOICE_STATE_UPDATE` to all guild members (sidebar shows occupants of every voice channel in every guild, live).

### 9.2 Join / switch / leave flow
1. Client calls `VoiceService.Join(channelId)`.
2. Hub checks: member, not banned, channel is voice, `user_limit`, instance limits.
3. Hub atomically: removes user's previous voice state (any guild), assigns new state (pending), mints LiveKit token (identity = userId, room = `vc_<channelId>`, grants: publish sources `[microphone, screen_share, screen_share_audio]` (+`camera` if enabled), subscribe, publish data; metadata = the voice session id; TTL 10 min — LiveKit refreshes tokens for connected clients), issues ICE servers from `IceServerProvider` (TTL 12 h). If the previous state was in another room, the hub removes that participant: LiveKit replaces duplicate identities only within one room.
4. Client disconnects any previous room, then `room.connect(url, token, { rtcConfig: { iceServers } })`.
5. LiveKit webhook `participant_joined` → state confirmed. No confirmation within 30 s → state cleared.
6. `participant_left` / `participant_connection_aborted` → state cleared (ignore if `voiceSessionId` differs: user already moved). Track webhooks carry only the participant SID, which the hub remembers when it confirms the connection. A participant whose session is not the user's current one is removed; one the hub has no state for is adopted if the user is a member (after a hub restart) and removed otherwise. Server mute and deafen are re-applied on every confirmation.
7. Same user joining from another device → LiveKit drops the old participant (duplicate identity); old client shows "Voice connected on another device".
8. Hub restart → rebuilds voice states from `ListParticipants` of all `vc_*` rooms. The sidecar stops with the hub, so in practice clients reconnect to the restarted LiveKit and are adopted from their webhooks.
9. Move (admin): the hub sets the target channel on the same session, removes the participant from the old room and sends `VOICE_FORCED{moved}`; the member's client joins the target (LiveKit's `MoveParticipant` is Cloud-only). Disconnect (admin) ends the state and removes the participant.

Switching channels is the same call; target ≤ 1 s. Use `Room.prepareConnection` on hover of a voice channel to pre-warm.

### 9.3 Mute / deafen semantics (Discord-like, exact)
- **Mute (mic)**: local mic track muted → nobody hears the user. Toggle works in or out of a channel; state persists across sessions and is applied on join.
- **Deafen (audio)**: implies mute; additionally disables all remote **microphone** publications (`setEnabled(false)` → SFU stops forwarding). App sounds (notifications, join/leave) still play. Screen-share audio follows its own tile volume; setting "Deafen also mutes stream audio" (default off).
- **Undeafen** restores the mute state from before deafening.
- Self states are sent via `UpdateVoiceSelfState` and shown to everyone (icons).
- **Server mute/deafen** (admin): hub revokes publish permission for `microphone` (and mutes the track) / revokes subscribe permission; user cannot override; red icons.
- Local per-user mute and per-user volume (0–200 %) from the context menu, persisted per user id locally.

### 9.4 Audio pipeline & settings
`getUserMedia` constraints from settings → optional processing chain (AudioWorklet): noise suppression (`none | browser | rnnoise` via `@sapphi-red/web-noise-suppressor`; browser NS is disabled when RNNoise is on) → input gain → **voice gate** (input sensitivity: automatic or manual dB threshold, 400 ms hangover; push-to-talk bypasses gate) → LiveKit `LocalAudioTrack` processor.
- Settings: input device, output device (`room.switchActiveDevice('audiooutput')`), input volume, output volume, mic test with level meter and loopback playback, input mode (voice activity / push-to-talk + key + release delay), echo cancellation, auto gain control, noise suppression mode.
- Encoding: mic Opus mono, 48 kbps default (instance cap ≤ 96 kbps), DTX on, RED on. Screen audio Opus stereo 128 kbps, DTX off.
- Remote playback through `<audio>` elements by default (keeps Chromium AEC effective). Volumes > 100 % route that user through WebAudio (note in UI: echo cancellation may degrade; headphones recommended).
- Speaking indicator: LiveKit active-speaker + local level, 400 ms hangover. Sounds: join/leave/mute/unmute/deafen/undeafen/stream start/stop (toggleable).
- Device hot-plug: listen to `devicechange`; fall back to default device if the selected one disappears.
- PTT: browser = while window focused (release on `blur`); desktop = global (shell bridge).

### 9.5 Screen sharing
Room options: `adaptiveStream: true`, `dynacast: true`.

| Preset | Resolution | FPS | Max bitrate (Text/Detail) | Max bitrate (Motion) |
|---|---|---|---|---|
| `720p30` | 1280×720 | 30 | 1.5 Mbps | 2.5 Mbps |
| `1080p30` | 1920×1080 | 30 | 2.5 Mbps | 4.5 Mbps |
| `1080p60` | 1920×1080 | 60 | 4 Mbps | 7 Mbps |
| `1440p30` | 2560×1440 | 30 | 4 Mbps | 6 Mbps |
| `1440p60` (max) | 2560×1440 | 60 | 6 Mbps | 10 Mbps |

- "Optimize for": **Text/Detail** (`contentHint: 'detail'`, `degradationPreference: 'maintain-resolution'`) or **Motion** (UE5 viewport, games: `contentHint: 'motion'`, `maintain-framerate`). Default Text.
- Codec: **Automatic** picks **AV1 for Text/Detail** (else VP9, else VP8) and **VP9 for Motion** (else VP8), from measurements ([ADR 0002](adr/0002-screen-share-codecs.md)); a **VP8 backup codec** serves viewers that cannot decode the choice. H.264 (hardware encoders), VP8, VP9 and AV1 can be chosen in Settings → Screen share.
- Every codec is published as **two simulcast layers**: full size and half size (≤ 30 fps, a quarter of the bitrate, at least 300 kbps); VP9 and AV1 use `L1T3` in each layer. Single-stream SVC is not used: LiveKit's client publishes a VP9 *screen share* with one spatial layer (and forces the motion hint) when asked for SVC, and dynacast pauses whole simulcast layers but not SVC layers.
- Grid tiles and thumbnails get the low layer; adaptive stream subscribes to the layer matching the rendered tile size and pauses off-screen tiles automatically.
- Capture constraints: `{ width: {ideal: W}, height: {ideal: H}, frameRate: {ideal: F, max: F} }`, cursor always; re-apply with `applyConstraints` after capture (desktop sources ignore some initial constraints).
- System/tab audio: browser checkbox; desktop via shell (Windows loopback). Request own-audio exclusion (`restrictOwnAudio` or equivalent where supported) so call voices are not re-captured; if unsupported, warn the sharer.
- One share per user per device. Channel cap on simultaneous shares (`max_shares_per_channel`): the client refuses past it, and the hub stops a share the media server reports past it by taking the screen share sources out of the participant's permissions for a moment. Instance and channel caps on the preset apply on the client (the media server cannot cap a publisher's resolution); the instance caps reach clients in `Ready.instance`.
- Stopping from the browser's native "Stop sharing" ends the share (`ended` event).
- Streaming flag in voice state is set by the hub from LiveKit `track_published/unpublished` (source `screen_share`), not by the client.
- Stats overlay (toggle `Ctrl+Shift+S` or tile menu): resolution, fps, codec, scalability layer, bitrate, packet loss, jitter, RTT, candidate type (host/srflx/prflx/relay), quality limitation reason. On the sharer's own preview it lists each sent layer and whether dynacast paused it.
- Measurements and the per-mode codec defaults: [ADR 0002](adr/0002-screen-share-codecs.md); `task bench:share` repeats them.

### 9.6 Camera (optional, cheap)
Same pipeline; `camera` source; default 720p30, simulcast. Off by default in instance settings.

---

## 10. Stage (voice channel view) & focus system

### 10.1 Tiles
- **User tile**: avatar on tinted background (color from user id), name, mute/deafen/server-mute icons, green speaking border (2 px inset), connection-quality icon on hover. Shows camera video when on.
- **Share tile**: video (`object-fit: contain`), red **LIVE** badge, sharer name, stream audio volume, hover toolbar: Focus · Fullscreen · Draw (pen) · Snapshot · Stats · Stop watching. Self-share tile shows "You are sharing" preview (can be hidden). Draw and Snapshot come with drawing (§11).
- Each share has one `<video>`, owned outside React (`ShareViews`, created by the voice controller so tiles can mount before the media engine loads). A tile shows it through a callback ref that moves the element in; a tile that changes place (grid, focus, filmstrip, fullscreen) gets the same element within one render, so it keeps its last frame instead of starting black: a screen capture sends a new frame only when the screen changes.

### 10.2 Modes (local, per user; never forced on others)
- **Grid**: all visible tiles equal size, normalized 16:9, layout chosen to maximize tile area for the container (algorithm below). Share tiles first (this device's own after the others), then people, each sorted by name; the speaking ring shows who talks. Toggle "Hide participants without video".
- **Focus**: one large tile + filmstrip (bottom; collapsible; horizontal scroll). Click any filmstrip tile to swap focus. Double-click a grid tile to focus it.
- **Fullscreen**: Fullscreen API on the stage container in Focus mode; chrome auto-hides after 3 s idle; Esc exits. Menus and tooltips render inside the stage while it is fullscreen.
- **Pop-out** (desktop: separate window; browser: Document Picture-in-Picture where supported) — Phase 11.
- Auto behavior: when the first share starts and the user is in Grid with no manual choice, switch to Focus on it ("auto-focus first share", setting default on). When the focused share ends, fall back to the next share or Grid. Media for a share arriving after the voice-state flag must not break focus (track `intendsShare` vs `hasVideo` separately): a share tile exists from the hub's streaming flag or from its media, whichever comes first. This device's own share is never focused by itself.
- Keyboard: `G` grid, `F` focus/fullscreen toggle, `←/→` cycle focus, `Esc` exit fullscreen/draw mode, `D` draw mode on focused share (with drawing), `Ctrl+Shift+S` stats.

### 10.3 Grid algorithm
For N tiles, container W×H, gap g, aspect a = 16/9: for each cols c in 1..N, rows r = ceil(N/c); tileW = min((W − (c−1)g)/c, ((H − (r−1)g)/r)·a); pick c maximizing tileW; center the last row. Pure function in `apps/web/src/features/stage/layout.ts` with unit tests.

### 10.4 Voice channel UI chrome
Top bar: channel name, participant count, layout toggle, "hide no-video", chat toggle (text-in-voice, chat phase). Bottom floating control bar: mic, deafen, share screen (split button: quality/optimize/audio; settings apply to the next share), camera (opt), drawing settings, fullscreen, disconnect.

---

## 11. Drawing on shared screens (Slack-style)

### 11.1 Behavior
- Anyone in the channel can draw on **any** share in that channel, including the sharer on their own share (desktop: overlay draw mode; browser: on their self-preview tile).
- The **sharer** can disable drawing for their share ("Allow drawing" toggle, default from their settings, default on). Channel setting `drawing_allowed` can disable it channel-wide (admins).
- Activate: pen button on a share tile (or `D` on the focused share). Cursor becomes a pen; left-drag draws; `Esc` or right-click exits. A compact toolbar appears on the tile: color, size, tool, "undo my last", "clear mine" (sharer also sees "clear all").
- **Lifespan (never infinite):** a stroke stays fully opaque while drawn and for `hold` after pointer-up, then fades over 800 ms. `hold` options 2/3/5/8 s (default 3 s), capped by instance setting (max 10 s). The author's choice travels with the stroke so every viewer sees the same timing.
- **Name tag**: author display name in a pill (author color) next to the pen tip while drawing and 1.5 s after.
- **Customization (per user)**: color (8 presets + custom hex; default auto-assigned from user id), size S/M/L, tool, hold time, "show name tags" on/off.
- Tools: **Pen** (smoothed freehand), **Highlighter** (3× width, 35 % alpha), **Arrow** (drag start→end, arrowhead), **Laser** (transient glowing dot + 600 ms trail, no persistence).
- Sharer sees strokes on their real screen through the desktop overlay (§16.5). In browser-only sharing, the sharer sees them on the self-preview tile.

### 11.2 Geometry
- Points are normalized `[0,1]²` relative to the **share's video frame content rect** (account for `object-fit: contain` letterboxing using `videoWidth/videoHeight` vs element box). Same mapping in the overlay (display bounds, DPI-aware).
- Width is relative to content height (`widthRel`, e.g. S=0.0025, M=0.004, L=0.007) so lines look identical at any tile size.
- Input uses Pointer Events with `getCoalescedEvents()`; points decimated (min distance 0.0008 normalized) and smoothed with midpoint quadratic curves when rendered.

### 11.3 Transport (`dislackso.annotation.v1`, LiveKit data packets, topic `annot`)
```
AnnotationPacket { oneof {
  StrokeBegin { stroke_id, target_track_sid, tool, color_rgba, width_rel, hold_ms }        // reliable
  StrokePoints{ stroke_id, seq, packed_points (uint16 x,y pairs scaled 0..65535), dt_ms[] } // lossy, every 16–33 ms
  StrokeEnd   { stroke_id, total_points, packed_points (full set, repairs lossy gaps) }       // reliable
  Undo        { stroke_id }                                                                  // reliable
  Clear       { target_track_sid, scope: MINE|ALL }                                         // reliable; ALL only from sharer
  Laser       { target_track_sid, x, y, ended }                                             // lossy
  ShareDrawingPolicy { target_track_sid, allowed }                                          // reliable, from sharer
}}
```
- Author identity comes from the LiveKit packet sender (never from payload). Late joiners do not receive history (strokes are short-lived by design).
- Target = share track SID; packets for unknown/ended tracks are dropped.
- Latency target: < 100 ms stroke-to-screen for peers near the host.

### 11.4 Rendering & anti-grief (`packages/annotation`)
- `AnnotationEngine` (pure TS, no React): stroke model, packet encode/decode, policy checks, timing.
- `CanvasRenderer`: one canvas per share surface, DPR capped at 2, rAF loop runs only while strokes/lasers exist.
- Receiver-side limits per author per target: ≤ 240 points/s, ≤ 8 concurrent strokes, ≤ 40 active strokes per target overall (oldest evicted), max 4,000 points per stroke. Violations dropped silently, counted in diagnostics.
- Used by web tiles and by the desktop overlay window (same package).

### 11.5 Snapshot
Tile action composes the current video frame + visible strokes onto a canvas → copy to clipboard / save PNG / (chat phase) post into the channel's text-in-voice chat.

---

## 12. Accounts & authentication

- **Register**: username (3–32, `[a-zA-Z0-9_.]`, unique case-insensitive), email, password (≥ 8, checked against a small common-password list), invite code when registration mode requires it. Display name defaults to username.
- **Registration modes** (instance setting): `invite_only` (default: valid guild invite or instance invite), `open`, `closed`.
- **Email verification**: required when SMTP is configured (link with single-use token, 24 h). Unverified users can log in only to a "verify your email" screen.
- **Password reset**: request always answers "if the account exists, an email was sent"; token single-use, 30 min; success revokes all sessions.
- **No-SMTP mode**: verification disabled automatically; instance admin can generate a reset link for a user in the admin panel; the hub logs a warning on boot.
- **Mailer**: `smtp` (STARTTLS/TLS; docs include Gmail app-password steps) and `log` (dev: prints emails). Templates: plain text + simple HTML, localized.
- **Sessions**: list with device/IP/last seen, revoke one/all others; password change revokes others.
- **Email change**: needs the password; a confirmation link (24 h) goes to the new address and opening it moves and verifies the account. Without SMTP the address changes at once.
- **Account deletion**: needs the password; owned guilds must be transferred or deleted first, and the last instance admin cannot leave. The account leaves every guild, its sessions end, and its row stays anonymized (username and email freed, profile cleared) and hidden from every lookup, so history shows it as a former member.
- **Instance admin**: created in setup; can promote others.

---

## 13. Guilds, channels, invites, moderation

### 13.1 Limits (instance settings, admin-editable)
`max_guilds_owned_per_user` (default 5) · `max_guild_members` (200) · `max_channels_per_guild` (50) · `default_voice_user_limit` (25) · `max_shares_per_channel` (6) · `max_share_preset` (`1440p60`) · `max_upload_mb` (100) · `stroke_hold_max_ms` (10000) · `mic_bitrate_max_kbps` (96) · `turn.monthly_budget_gb` (900).

### 13.2 Guild visibility
Guilds are private: visible only to members. Joining only via invite. Banned users cannot preview or accept invites.

### 13.3 Invites
8-char code from `ABCDEFGHJKLMNPQRSTUVWXYZ23456789`; options: expiry (30 min, 1 h, 6 h, 1 d, 7 d, never), max uses (1, 5, 10, 25, ∞). Link `https://<instance>/invite/<code>` → landing page (guild icon, name, member/online count) → login/register → join. Desktop deep link `dislackso://invite/<code>?i=<instance-host>`.

### 13.4 Permission matrix (`domain.Policy`)
| Action | Owner | Admin | Member |
|---|---|---|---|
| Update guild name/icon | ✓ | ✓ | |
| Delete guild, transfer ownership, set/unset admins | ✓ | | |
| Create/rename/reorder/delete channels & categories, channel settings | ✓ | ✓ | |
| Create invites | ✓ | ✓ | ✓ (instance setting "members can invite", default on) |
| Revoke invites, view invite list | ✓ | ✓ | own |
| Kick / ban / unban (cannot target owner; admins cannot target admins) | ✓ | ✓ | |
| Delete any message (chat phase) | ✓ | ✓ | own only |
| Server mute/deafen, disconnect, move members in voice | ✓ | ✓ | |
| View audit log | ✓ | ✓ | |
| Leave guild | must transfer/delete | ✓ | ✓ |

Kick/ban while connected to voice → hub removes the participant from LiveKit and sends `GUILD_DELETE{reason}` to the target. Every moderation action writes `audit_log`.

---

## 14. Web client architecture (`apps/web`)

```
src/
  app/            bootstrap, router (TanStack Router), providers, error boundaries
  core/
    api/          Connect transport + interceptors (Unauthenticated → login, retries on idempotent reads)
    gateway/      GatewayClient (lifecycle, heartbeat, resume, backoff) → GatewayDispatcher → stores
    media/        VoiceSession (wraps livekit Room), DeviceManager, AudioProcessing, ScreenShareController,
                  QualityPresets, StatsCollector, Sounds
    annotation/   adapter: AnnotationEngine ↔ LiveKit data channel ↔ tile renderers / desktop overlay bridge
    platform/     PlatformBridge interface: WebPlatform | DesktopPlatform (window.desktop)
    i18n/         en, pt-BR
  stores/         Zustand: session, entities(users, guilds, channels, members), presence, voiceStates,
                  voiceSession (local call state), stage (mode/focus per channel), settings (persisted), ui
  features/       auth, guild, channel, voice, stage, drawing, members, profile, settings, invites,
                  moderation, admin, chat(opt), threads(opt), social(opt)
  ui/             design-system primitives (Button, IconButton, Modal, Menu, ContextMenu, Tooltip, Popover,
                  Tabs, Switch, Slider, Select, Avatar, Badge, Toast, ScrollArea, Skeleton) on Radix
  styles/         tokens.css, themes.css, base.css
```
Rules:
- `core/*` is framework-agnostic (no React imports). React reads stores; components call feature actions; actions call `core`.
- Only `GatewayDispatcher` mutates entity stores from push events; RPC results also flow through the same reducers (idempotent upserts, versioned by `updated_at`).
- TanStack Query only for on-demand data (message history pages, search, audit log, bans list); gateway events patch its caches.
- Virtualized lists (`@tanstack/react-virtual`) for messages and large member lists.
- Settings persisted locally (versioned schema with migrations) and synced to the server for cross-device prefs (theme, drawing prefs); device-specific settings (devices, volumes) stay local.
- Strict TS (`strict`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`).

---

## 15. UI specification (Discord-like, original branding)

Use the current Discord desktop layout as the reference for structure, spacing and interactions. Do **not** use Discord's name, logo, illustrations, sounds, font (gg sans) or icon set. Font: **Inter** (UI) + **JetBrains Mono** (code). Icons: Lucide.

### 15.1 Frame
```
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│ Title/top bar (32px): current guild ▸ channel · search · inbox · help · (desktop: window controls) │
├──────┬────────────────────┬───────────────────────────────────────────────┬──────────────┤
│ Rail │ Channel sidebar    │ Main header (48px): #channel / 🔊 voice name, │ Member list  │
│ 72px │ 240px (resizable   │ topic, toolbar (threads, pins, members, search)│ 240px        │
│      │ 200–420)           ├───────────────────────────────────────────────┤ (toggle)     │
│ Home │ Guild header ▾     │                                               │ Owner        │
│ ──── │ ▸ CATEGORY         │  Stage (voice) or Chat (text) or Home         │ Admins — N   │
│ (G)  │   # text           │                                               │ Online — N   │
│ (G)  │   🔊 voice         │                                               │ Offline — N  │
│ (+)  │     ◉ user  🔇 LIVE│                                               │              │
├──────┴────────────────────┤                                               │              │
│ Voice panel (when connected): "Voice Connected" · channel/guild · quality · │              │
│   disconnect · share · noise suppression                                   │              │
│ User panel: avatar+status · name/status · [mic] [headphones] [⚙]          │              │
└───────────────────────────┴───────────────────────────────────────────────┴──────────────┘
```
- **Rail**: Home (DMs/friends) button, separator, guild icons 48 px (circle → rounded square on hover/selected), left pill indicator (selected = tall, unread = dot, hover = medium), mention badge, `+` create/join guild. Drag to reorder (local pref). Tooltip with guild name.
- **Channel sidebar**: guild header with dropdown (Invite People, Guild Settings, Create Channel, Create Category, Leave). Categories collapsible (uppercase 12 px semibold). Text channel rows (`#`), voice rows (speaker icon) with nested occupants (24 px avatar, name, icons: muted, deafened, server-muted in red, LIVE badge, camera). Hover row actions: invite, settings (admins). Unread bold + mention badge. Drag-and-drop reordering for admins. Clicking a voice channel joins it (and opens the stage); clicking it again while connected opens the stage.
- **User panel** spans the bottom of rail + sidebar (as in the current Discord layout): avatar with status dot (click → status menu: online/idle/dnd/invisible, custom status, profile), name + status line, mic and headphones toggles (red slash when active, work outside calls), settings gear. Right-click mic/headphones → device quick-switch menu.
- **Voice panel** above it while connected: green "Voice Connected" (yellow while reconnecting, red on failure), channel/guild name (click → go to stage), quality bars (click → connection details: ping, packet loss, path direct/relay, codec), disconnect, share screen, noise-suppression toggle.
- **Member list**: groups Owner / Admins / Online / Offline with counts; rows with avatar + status dot, display name, crown (owner) / shield (admin) icon, custom status. Click → profile popout (avatar, display name, username, member since, guild role, actions). Right-click → context menu: Profile, Mention, Message (opt), Local volume slider + local mute (when in same voice channel), [admin] Server mute, Server deafen, Disconnect, Move to ▸, Make/Remove admin (owner), Kick, Ban, Copy user ID (developer mode).
- **Connection banner**: top banner "Reconnecting…" when gateway is down > 2 s.

### 15.2 Screens & modals
Auth (login, register with invite code, verify-email pending with resend, reset request, reset form) on a centered card over an original abstract background · Invite landing · Home (friends/DMs in social phase; before that, guild list + "create/join" empty state) · Guild text channel (chat phase) · Voice stage (§10) · **User Settings** (full-screen modal, left nav): My Account, Profile, Voice & Audio, Screen Share, Drawing, Appearance (theme Dark/Onyx/Light, density compact/default/spacious, font scale, reduced motion), Notifications, Keybinds, Language, Advanced (developer mode, stats hotkey, hardware acceleration (desktop)), Sessions · **Guild Settings**: Overview, Channels, Members, Invites, Bans, Audit Log, Danger zone (transfer, delete with name confirmation) · **Channel Settings**: name, topic, user limit, drawing allowed, max share preset · **Instance Admin**: Overview (online, rooms, versions), Connectivity (exposure status/URL, self-test, suitability, % relayed), Media (LiveKit status, caps), TURN (provider, usage, budget), Email (SMTP + test), Registration & limits, Users, Backups, Logs.

### 15.3 Theme tokens (CSS variables; defaults, tune in Phase 2)
| Token | Dark | Onyx | Light |
|---|---|---|---|
| `--bg-rail` | #1e1f22 | #070708 | #e3e5e8 |
| `--bg-sidebar` | #2b2d31 | #111113 | #f2f3f5 |
| `--bg-main` | #313338 | #18181b | #ffffff |
| `--bg-floating` | #111214 | #000000 | #ffffff |
| `--bg-input` | #383a40 | #222226 | #ebedef |
| `--text-normal` | #dbdee1 | #e3e3e6 | #313338 |
| `--text-muted` | #949ba4 | #8e8e96 | #5c5e66 |
| `--accent` | #5b6cf0 (own brand, changeable) | same | same |
| `--green / --yellow / --red` | #23a55a / #f0b232 / #f23f43 | same | same |
Radii: 4/8/16; message avatar 40 px; voice occupant avatar 24 px; stage tile radius 8 px, gap 8 px.

### 15.4 Global shortcuts (rebindable)
`Ctrl+Shift+M` mute · `Ctrl+Shift+D` deafen · `Ctrl+K` quick switcher · `Ctrl+Shift+S` stats overlay · `Alt+↑/↓` previous/next channel · `Esc` close modal/exit draw/fullscreen · PTT key (unset by default).

### 15.5 Quality bar
Smooth 60 fps interactions, no layout shift when voice states change, skeletons instead of spinners, keyboard navigable, visible focus rings, `prefers-reduced-motion` respected, all strings via i18n.

---

## 16. Desktop shell (`apps/desktop`, Electron + TypeScript, electron-vite)

### 16.1 Responsibilities
Instance launcher (saved instances, each in its own `persist:` partition) → loads `https://<instance>/` in the main window. Native features via a typed preload bridge exposed **only to saved instance origins**.

### 16.2 Security
`contextIsolation: true`, `sandbox: true`, `nodeIntegration: false`, strict navigation guard (external links → OS browser), permission handler grants media/display-capture/notifications only to instance origins, no remote module, CSP for local shell pages, bridge methods validate arguments.

### 16.3 Bridge (`window.desktop`, versioned `bridgeVersion`)
```ts
interface DesktopBridge {
  bridgeVersion: number;
  platform: 'win32' | 'darwin' | 'linux';
  screen: { getSources(): Promise<CaptureSource[]>; choose(sourceId: string, opts: { audio: boolean }): void; cancel(): void; onPickRequested(cb: () => void): Unsubscribe; };
  overlay: { open(target: OverlayTarget): Promise<void>; close(): Promise<void>; send(packet: Uint8Array): void; setDrawMode(on: boolean): void; onLocalPacket(cb: (p: Uint8Array) => void): Unsubscribe; };
  hotkeys: { register(binding: HotkeyBinding[]): Promise<void>; onAction(cb: (a: HotkeyAction, pressed: boolean) => void): Unsubscribe; };
  notify(n: NativeNotification): void; setBadge(count: number): void; flashFrame(): void;
  idle: { onIdleChange(cb: (idle: boolean) => void): Unsubscribe };
  host: { status(): Promise<HostStatus>; start(): Promise<void>; stop(): Promise<void>; openSetup(): void };
  app: { version(): string; checkForUpdates(): Promise<UpdateInfo>; openExternal(url: string): void };
}
```

### 16.4 Screen picker & system audio
`session.setDisplayMediaRequestHandler` → bridge emits `onPickRequested` → web app renders its own picker (screens/windows tabs with live thumbnails from `desktopCapturer`, "share audio" checkbox) → `choose(id, {audio})`. Windows: `audio: 'loopback'` for system audio (screen sources). macOS/Linux system audio: best effort per current Electron capabilities; otherwise hide the checkbox. On macOS 15+, optionally use the system picker.

### 16.5 Drawing overlay (sharer's real screen)
- One transparent, frameless, non-focusable, always-on-top (`screen-saver` level), click-through (`setIgnoreMouseEvents(true, {forward: true})`) window covering the shared display; `setContentProtection(true)` so it is excluded from capture (verify per OS; macOS 15+ capture may ignore it — acceptable).
- Renders with `@dislackso/annotation` from packets relayed via IPC by the main renderer.
- Floating mini-toolbar (draggable, auto-hide): draw mode toggle, color, clear all, allow-drawing toggle, stop sharing. Draw mode makes the overlay capture the mouse until `Esc`/toggle; the sharer's strokes are sent back through the bridge to the room.
- Window shares: overlay follows the target window bounds (polling via `get-windows`, 10 Hz); hidden when the window is minimized (Phase 7b).

### 16.6 Other native features
Global hotkeys including PTT key-down/key-up via `uiohook-napi` (macOS needs Accessibility permission; Wayland unsupported → fall back to focused-window PTT) · tray icon (mute/deafen/quit) · close-to-tray · start at login · native notifications + taskbar badge/overlay icon · deep links `dislackso://` · window state persistence · hardware acceleration toggle · auto-update via `electron-updater` from GitHub Releases (repo must be public; macOS unsigned builds cannot auto-update — show "download new version" instead) · **Host mode**: bundles the hub binary; "Host an instance" spawns it with a data dir under app data and opens the setup wizard.

---

## 17. Optional features (designed now, built in later phases)

### 17.1 Text channels & messaging
- Send/edit/delete, replies (quote preview), mentions (`@user`, `@here` for admins), channel links, reactions (Unicode emoji, Twemoji rendering with attribution), typing indicators, unread markers + "jump to present", read states synced across devices, message grouping (same author within 7 min), compact/cozy modes, infinite scroll both directions with anchor preservation, optimistic send with nonce reconciliation, failed-send retry.
- **Markdown (Discord-flavored)**: bold, italic, underline `__`, strikethrough, spoiler `||`, inline code, fenced code with syntax highlighting (Shiki, languages lazy-loaded), block quotes, headers `#`–`###`, lists, masked links `[text](url)`, auto-linking, `<t:unix:format>` timestamps, emoji shortcodes. Parser → AST → React renderer, no raw HTML. Pure module with tests.
- Link embeds: hub fetches OpenGraph (SSRF-guarded, cached) → embed card; YouTube embeds play inline.
- **Text-in-voice**: every voice channel has its own chat panel in the stage.

### 17.2 Attachments
Drag & drop / paste / button; upload progress; images (lightbox gallery, zoom), videos (inline player, poster frame generated client-side at upload), audio player, generic files with icon/size/download; code/text files preview. Client computes dimensions/duration and thumbnail before upload; hub validates and re-encodes images.

### 17.3 Threads
Created from a message ("Create Thread") or from the composer `+` menu (which also hosts "Upload file", "Create poll" later). Thread = channel `type=thread` with `parent_id` and starter message; opens in a right-side panel; thread list per channel (active/archived); auto-archive after 7 days idle.

### 17.4 Friends & DMs
Friend requests (send by username, accept, decline, cancel), remove, block (hides messages, blocks DMs/requests), DM any user who shares a guild or is a friend, close DM (hide), delete own messages. DMs are text-only in v1. Home view: Friends tabs (Online, All, Pending, Blocked) + DM list.

### 17.5 Search
FTS5 over messages with filters `from:`, `in:`, `has:file`, `before:`, `after:`.

### 17.6 Incoming webhooks (dev-team friendly)
Per text channel, Discord-compatible JSON subset (`content`, `username`, `avatar_url`, `embeds`) so CI/GitHub tools that post to Discord webhooks can target this app.

### 17.7 Portable instance (multi-host, independent storage)
Goal: whoever is online can host the same instance.
- DB: Litestream continuous replication of `data.db` to S3-compatible storage (Cloudflare R2 / Backblaze B2 free tiers), sync interval tuned to stay inside free operation quotas.
- Files: `BlobStore` S3 implementation.
- **Lease**: `lease.json` object written with conditional PUT (If-Match / If-None-Match), renewed every 10 s, TTL 30 s. A starting hub restores the latest replica, acquires the lease, then serves; standby hubs refuse to serve writes and show "Instance is hosted by <name>".
- Stable URL requires `cloudflare-tunnel` (all hosts run the same tunnel token; only the lease holder starts the connector). With Funnel, each host has its own URL; the desktop shell stores multiple endpoints per instance and picks the healthy one.

### 17.8 Web Push
VAPID-based push for mentions/DMs when no tab is open (browser push services are free).

---

## 18. Observability, testing, CI/CD, packaging

- **Logs**: `slog` JSON to rotating files + console; request IDs; sidecar logs captured. Admin panel log viewer (tail + filter). "Copy diagnostics" bundles versions, settings (secrets redacted), self-test, recent errors.
- **Health**: `/healthz` (process), `/readyz` (DB + LiveKit + exposure).
- **Client diagnostics**: stats overlay, gateway frame inspector (dev), "Report a problem" that attaches client logs + WebRTC stats snapshot.
- **Tests**:
  - Go: unit tests per service with an in-memory SQLite and fake ports; repository tests on real SQLite; gateway tests (resume/replay, rate limits); integration tests that start a real `livekit-server --dev` and exercise Join → webhook reconciliation → moderation enforcement.
  - TS: Vitest for pure modules (layout, annotation geometry/encoding/policy, markdown, reducers, settings migrations); component tests for critical UI.
  - E2E: Playwright with Chromium flags `--use-fake-ui-for-media-stream --use-fake-device-for-media-stream --auto-select-desktop-capture-source=Entire screen`: 3 users register → create guild → invite → join voice → mute/deafen visibility → share screen → second user focuses + draws → third user sees stroke → admin kicks user.
- **CI (GitHub Actions)**: lint (golangci-lint, ESLint, Prettier check), `buf lint` + `buf breaking` (after v1.0) + generated-code freshness, Go tests (ubuntu + windows), web tests, E2E (ubuntu, LiveKit binary cached), build.
- **Release**: tag `v*` → hub archives for windows/macos/linux × amd64/arm64 (hub + pinned `livekit-server` + licenses + checksums), desktop installers (Windows NSIS, macOS dmg unsigned, Linux AppImage + deb), GitHub Release with notes. Web client is embedded in the hub build (`//go:embed`).
- **Versioning**: semver for hub (includes web) and desktop shell independently; `/api/version` exposes `hubVersion`, `minBridgeVersion`; the web app degrades features when the bridge is older.

---

## 19. Repository layout

```
/
├─ CLAUDE.md  README.md  LICENSE  Taskfile.yml  .editorconfig  .gitignore
├─ docs/      ARCHITECTURE.md  ROADMAP.md  STATUS.md  HOSTING.md  adr/
├─ proto/     buf.yaml  buf.gen.yaml  dislackso/...
├─ hub/       go.mod
│   ├─ cmd/hub/main.go
│   ├─ gen/                         generated protobuf/connect code
│   ├─ internal/app/                composition root
│   ├─ internal/config/
│   ├─ internal/domain/             entities, errors, policy, snowflake
│   ├─ internal/ports/              interfaces
│   ├─ internal/service/<area>/
│   ├─ internal/adapter/{sqlite,mail,livekit,ice,exposure,portmap,stun,hostos,blob,ratelimit,password}/
│   ├─ internal/transport/{rpc,gateway,web,ingress,media,files,wire}/
│   ├─ internal/events/  internal/sidecar/
│   └─ webdist/                     embedded web build (gitignored output, embed.go committed)
├─ apps/web/        Vite React app
├─ apps/desktop/    Electron shell
├─ packages/contracts/    generated TS from proto + tiny helpers
├─ packages/annotation/   engine + renderer shared by web and desktop overlay
├─ packages/config/       shared tsconfig/eslint/prettier presets
└─ e2e/             Playwright
```
Toolchain: Go (latest stable), Node LTS, pnpm, Buf CLI, `task`. Pin versions in `Taskfile.yml`/`package.json`/`go.mod`; dev scripts download `livekit-server` into `.tools/`.

---

## 20. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Host behind CGNAT without IPv6/UPnP → every client relays | Self-test flags it; TURN budget guard; admin guidance to pick another host or use an always-free VM |
| LiveKit behind NAT is not an officially documented topology | Full ICE + TURN relay path covers it; relay self-test proves it per host; `MediaProvider` can point to an external LiveKit |
| Tailscale Funnel bandwidth/beta | Used only for HTTP; Cloudflare named tunnel alternative for heavy file use |
| Free tiers change | Every external service sits behind an interface with ≥ 2 implementations or a degraded mode |
| Browser AEC degraded by WebAudio | Default `<audio>` playback; only boosted users go through WebAudio; warn |
| macOS unsigned builds | Document Gatekeeper bypass; no auto-update on macOS |
| Wayland limits (global hotkeys, overlay) | Fall back to focused-window PTT; overlay best effort |
| Host PC sleeps | Keep-awake while hosting; clients show "instance unreachable" with retry |
| Email deliverability | Gmail app password default guide; SMTP provider pluggable; no-SMTP mode |
