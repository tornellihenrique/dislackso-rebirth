# Roadmap

One phase per working session. A phase is done only when every acceptance box is checked, `task check` is green, `docs/STATUS.md` is updated, and a manual test script for the phase is written at the end of `docs/STATUS.md`. Then stop and wait for review.

Must-haves complete at **Phase 8 → v1.0**. Phases 9–11 are the optional features.

Global Definition of Done (every phase):
- Lint, typecheck, unit tests, and relevant integration/E2E tests pass locally and in CI.
- No business logic in transport layers or React components; new external services behind a port/interface.
- Contract changes made in `proto/` first, code regenerated and committed.
- New user-facing strings in i18n (`en` + `pt-BR`).
- Docs updated when behavior or decisions change (`ARCHITECTURE.md` section + `STATUS.md`).

---

## Phase 0 — Foundation
Deliverables
- Monorepo per ARCHITECTURE §19: pnpm workspaces, Go module, `Taskfile.yml` (`dev`, `gen`, `check`, `test`, `build`, `e2e`, `tools`), `.editorconfig`, lint/format configs (golangci-lint, gofumpt, ESLint strict, Prettier).
- `proto/` with Buf config and a `HealthService` + empty `gateway.proto`; `task gen` produces Go + TS.
- Hub skeleton: config loading, slog logging, graceful shutdown, HTTP mux serving the embedded web build with SPA fallback, `/healthz`.
- Web skeleton: Vite + React + TS strict + Tailwind v4 + Router + i18n + tokens/themes files; calls `HealthService` over Connect.
- `task tools` downloads pinned `livekit-server` into `.tools/` for the dev OS.
- GitHub Actions: lint, gen freshness, Go tests (ubuntu, windows), web tests, build.
- `docs/STATUS.md` created; `docs/adr/0001-record-architecture.md` pointing to the decision ledger.

Acceptance
- [ ] `task dev` runs hub + Vite with proxy; browser shows the app shell and a live health check.
- [ ] `task build` produces a single hub binary that serves the built web app.
- [ ] CI green on a PR.

## Phase 1 — Accounts & instance setup
Deliverables
- Data dir resolution per OS, `config.yaml`/`secrets.yaml`, SQLite (WAL, single writer + readers), goose migrations, sqlc, Snowflake IDs, `TxRunner`.
- Domain + services: `auth`, `account`, `instance` (settings with defaults from ARCHITECTURE §13.1).
- argon2id hasher, session tokens (hashed), cookie handling, rate limiter, `Mailer` (`smtp`, `log`), localized email templates.
- Setup wizard (localhost-only + one-time setup token): instance name, instance-admin account, SMTP (optional, with "send test email"). Exposure/TURN steps are placeholders until Phase 5.
- Registration modes (`invite_only` default, `open`, `closed`), instance invites, verification, password reset, no-SMTP mode, session list/revoke, profile (display name), status preference.
- Web: auth screens (login, register + invite code, verify pending, reset request/form), setup wizard UI, minimal settings (My Account, Profile without avatar, Sessions).

Acceptance
- [ ] Fresh data dir → wizard → admin created → login works; setup unreachable afterwards and from non-localhost.
- [ ] Register in `invite_only` requires a valid instance invite; `open`/`closed` behave correctly.
- [ ] With SMTP: verification and reset emails arrive, tokens single-use and expiring; reset revokes sessions. Without SMTP: verification auto-disabled, admin can create a reset link (RPC available; UI may be minimal).
- [ ] Unknown user and wrong password return the identical error; rate limits trigger `ResourceExhausted`.
- [ ] Unit tests for every auth/account service path; repository tests on real SQLite.

## Phase 2 — Guilds, channels, realtime shell, basic moderation
Deliverables
- Services: `guild`, `channel` (categories, voice, text placeholders), `invite`, `moderation` (kick, ban, unban, admins, transfer ownership), `domain.Policy` with full matrix tests, audit log writes.
- Blob store (`LocalFS`), `/api/uploads`, `/files/*`, avatar + guild icon upload with server-side re-encode.
- Gateway (ARCHITECTURE §8): Hello/Identify/Resume/Ready, heartbeats, replay buffer, close codes, topics, presence (online/idle/dnd/invisible), event catalog for this phase, Origin check, frame rate limit.
- Web: `GatewayClient` + `GatewayDispatcher` + entity stores; Discord-like frame (§15.1): rail, channel sidebar (categories, drag reorder for admins), user panel, member list with groups/presence, profile popout, context menus, guild create/join modals, invite landing page, guild settings (Overview, Channels, Members with kick/ban/admin, Invites, Bans, Audit Log, Danger zone), channel settings, user settings (Profile with avatar crop, Appearance with themes/density), connection banner.

Acceptance
- [ ] Two browsers: creating a guild/channel/invite, joining, renaming, reordering, role changes and presence changes appear on the other browser in < 500 ms without refresh.
- [ ] Killing the network for 30 s then restoring resumes the gateway without a full reload (verified by seq replay test); after 3 min it re-identifies cleanly.
- [ ] `max_guilds_owned_per_user` enforced; non-members cannot see a guild in any API or gateway event (tests).
- [ ] Kick/ban remove the guild from the target's UI immediately; banned user cannot accept new invites.
- [ ] Policy matrix fully unit-tested (owner/admin/member × every action).

## Phase 3 — Voice
Deliverables
- Sidecar supervisor (start/health/restart/backoff, log capture, Job Object on Windows), generated `livekit.yaml` (§4.3), LiveKit adapter for `MediaProvider`, `/lk/*` reverse proxy, webhook endpoint (localhost + signature verified).
- `voice` service: Join/Leave/Move, server mute/deafen, disconnect; in-memory voice states; reconciliation from webhooks and from `ListParticipants` at boot; single connection per user.
- `IceServerProvider` with `none` + `static` implementations (Cloudflare comes in Phase 5).
- Web `core/media`: `VoiceSession`, `DeviceManager`, audio pipeline (noise suppression modes, gain, voice gate, PTT in focused window), sounds, active speaker, reconnect states.
- UI: voice occupants in sidebar for every guild (icons for mute/deaf/server mute), voice panel (connected/reconnecting, quality, disconnect), mic/deafen toggles working in and out of calls with exact semantics (§9.3), per-user volume/local mute, Voice & Audio settings with mic test and meter, admin context actions (server mute/deafen, disconnect, move).

Acceptance
- [ ] Three users on LAN in two channels: audio isolated per channel; switching channels < 1 s; sidebar in other guilds shows occupants live.
- [ ] Deafen mutes all remote mics and own mic; undeafen restores previous mute; states persist across reload and apply on join; others see icons update live.
- [ ] Server mute cannot be bypassed by the client (integration test against real LiveKit).
- [ ] Killing a client process clears its voice state within `departure_timeout`; restarting the hub restores voice states from LiveKit.
- [ ] Joining from a second device moves voice there and the first shows the "another device" notice.
- [ ] Device unplug falls back to default without dropping the call.

## Phase 4 — Screen share & stage
Deliverables
- `ScreenShareController`: presets (§9.5), optimize-for Text/Motion, codec selection (VP9 SVC default + VP8 backup, H.264/AV1 advanced), tab/system audio (browser), caps from instance/channel, streaming flag via webhooks.
- Stage (§10): user/share tiles, Grid algorithm (pure + tests), Focus with filmstrip, Fullscreen with auto-hiding chrome, auto-focus first share, keyboard shortcuts, hide-no-video, stream volume, self-preview.
- Stats overlay and connection details popover.

Acceptance
- [ ] 1440p60 Text share of a code editor is crisp in Focus on a second machine (stats show 2560×1440 and ~60 fps on a healthy LAN); Motion mode on a UE5 viewport keeps ≥ 55 fps.
- [ ] With 3 simultaneous shares and 5 viewers in Grid, viewers receive low layers (stats) and the sharer's dynacast pauses unused layers.
- [ ] Focus/unfocus/fullscreen/grid transitions never show a black or frozen tile; share ending moves focus correctly.
- [ ] Decision on default codec per mode recorded in an ADR with measured CPU/bitrate numbers.

## Phase 5 — Hosting & connectivity (team dogfood milestone)
Deliverables
- Exposure providers (§4.2): `tailscale-funnel` (tsnet, login URL surfaced, Funnel/HTTPS guidance), `cloudflare-tunnel` (on-demand pinned cloudflared), `cloudflare-quick`, `direct` (CertMagic), `lan`; trusted client IP per provider.
- Port mapping (UPnP/NAT-PMP/PCP) for UDP 7882/TCP 7881 with renew/cleanup.
- `IceServerProvider`: `cloudflare` (minted credentials, port-53 filtered), `coturn-rest`; per-join issuance.
- `hub doctor` + admin Connectivity page: checks §4.4, browser relay test, suitability rating, guidance.
- Client telemetry over gateway; usage estimates; TURN budget guard.
- Keep-awake while hosting, run at login, Windows firewall helper.
- Wizard steps for exposure, TURN, connectivity test.
- Release workflow: hub archives (win/mac/linux × amd64/arm64) with pinned `livekit-server`, checksums.
- `docs/HOSTING.md`: linear steps for hosts (Funnel path, Cloudflare path, Gmail SMTP, Cloudflare TURN key, troubleshooting by self-test result).

Acceptance
- [ ] A teammate on a different ISP joins via the Funnel URL from a browser, hears voice and sees a 1080p60 share.
- [ ] With the host's router UPnP disabled and TURN configured, a client forced to `relay` connects (relay test passes) and admin panel shows the relayed session.
- [ ] Switching exposure provider in the admin panel works without data loss; quick-tunnel URL change is surfaced.
- [ ] TURN budget guard stops issuing credentials past the budget (test with a tiny budget).
- [ ] Release artifacts run on a clean Windows machine by double-click → wizard opens.

## Phase 6 — Drawing on shares
Deliverables
- `packages/annotation`: model, protobuf packet codec, policy/anti-grief, timing, `CanvasRenderer`, geometry utils; full unit tests.
- Web adapter over LiveKit data packets (lossy points, reliable begin/end/undo/clear/policy).
- Tile drawing mode + toolbar (color, size, tool: pen/highlighter/arrow/laser, undo mine, clear mine/all), name tags, hold/fade timing, Drawing settings page, sharer "allow drawing" toggle, channel `drawing_allowed`.
- Sharer draws on own self-preview in browser.
- Snapshot (clipboard/PNG).

Acceptance
- [ ] Two viewers draw simultaneously on two different shares; every participant sees identical strokes aligned to the content at any tile size, letterboxed or not.
- [ ] Strokes hold for the author's chosen time then fade; nothing persists beyond `stroke_hold_max_ms` + fade.
- [ ] Sharer disabling drawing hides pen controls for others within 500 ms; flood test (synthetic 2,000 points/s) is throttled without UI jank.
- [ ] Measured stroke latency < 100 ms on LAN (instrumented test).

## Phase 7 — Desktop shell
Deliverables
- Electron app per §16: instance launcher with partitions, security hardening, typed bridge with `bridgeVersion`, web `DesktopPlatform` implementation.
- Custom screen/window picker with thumbnails, Windows system audio loopback, own-audio exclusion where supported.
- Drawing overlay for display shares (click-through, content-protected, toolbar, sharer draw mode); 7b: window-share overlay tracking.
- Global hotkeys incl. PTT (uiohook-napi), tray, close-to-tray, start at login, notifications, badge, deep links, window state, hardware acceleration toggle.
- Auto-update from GitHub Releases (Windows/Linux); macOS manual update notice.
- Host mode: bundled hub, start/stop, open wizard, status in tray.
- Release workflow for installers (NSIS, dmg, AppImage, deb).

Acceptance
- [ ] Sharer on Windows sees teammates' strokes on the real screen in real time, and viewers do not see the overlay duplicated in the video.
- [ ] Global PTT works while another app is focused (Windows; macOS with Accessibility permission).
- [ ] System audio share on Windows does not include other participants' voices (or the warning is shown when exclusion is unsupported).
- [ ] Host mode from a fresh install brings a working instance online without a terminal.

## Phase 8 — Admin, polish, hardening → v1.0
Deliverables
- Instance admin panel complete (§15.2): users (disable, reset link, promote), registration & limits, email, media caps, TURN, backups (nightly `VACUUM INTO`, retention, manual backup/restore), log viewer, diagnostics bundle.
- Notifications settings, keybinds page, language switch, density/font scale, accessibility pass, empty/loading/error states everywhere.
- E2E suite (§18) green in CI; load test: 12 participants, 4 shares on one host machine with CPU/bandwidth report.
- Security review checklist (§5.4) verified; `buf breaking` enabled from v1.0.

Acceptance
- [ ] Full E2E scenario passes in CI.
- [ ] Restore from a nightly backup on another machine yields an identical instance.
- [ ] Every must-have from the product brief is demonstrably working (checklist in STATUS.md).

---

## Optional phases

### Phase 9 — Text chat
Text channels, text-in-voice, messages (send/edit/delete/reply/mentions), Discord-flavored markdown module with tests, code highlighting, typing indicators, read states/unreads synced, reactions, attachments with image/video/audio viewers, link embeds (SSRF-guarded), snapshot-to-chat, desktop/browser notifications for mentions.

### Phase 10 — Threads, friends & DMs, search, webhooks
Threads (from message or composer `+` menu, side panel, auto-archive), friends/blocks/DMs (text), FTS5 search with filters, Discord-compatible incoming webhooks, pins.

### Phase 11 — Extras
Portable instance (Litestream + S3 blob store + lease + multi-endpoint shell), camera, pop-out/PiP, hub self-update, Web Push.
