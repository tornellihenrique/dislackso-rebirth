# Status

## Current phase

**Phase 7 — Desktop shell: in progress.** Built: the desktop app, its screen picker, the drawing
overlay on the shared screen, installers for Windows, macOS and Linux, and updates from GitHub
releases. Next in this phase: window-share overlay, global hotkeys and push-to-talk, tray and host
mode. Phase 6 (drawing) is merged and in use.

### Phase 7 acceptance

- [x] Sharer on Windows sees teammates' strokes on the real screen in real time, and viewers do not
      see the overlay duplicated in the video. Playwright drives the desktop app (`task
      e2e:desktop`): a viewer in a browser draws on the app's screen share, the overlay window
      (always on top, protected from capture, not focusable) shows the stroke, and the viewer's
      received video has none of the stroke's pixels where the overlay draws it. With capture
      protection turned off the same check finds 1,152 of them.
- [ ] Global PTT works while another app is focused (Windows; macOS with Accessibility
      permission). Not built yet.
- [ ] System audio share on Windows does not include other participants' voices (or the warning is
      shown when exclusion is unsupported). The picker offers the computer's sound (loopback) on
      Windows; excluding the call's voices is not built yet.
- [ ] Host mode from a fresh install brings a working instance online without a terminal. Not built
      yet.

### Phase 6 acceptance

- [x] Two viewers draw simultaneously on two different shares; every participant sees identical
      strokes aligned to the content at any tile size, letterboxed or not. Playwright: two people
      share and, in windows of different sizes, draw on each other's share at the same moment;
      each sharer's own tile shows the other's line on the same spot of the content (pixels read
      back from the canvas) and nothing next to it. Unit tests cover the content rect and the
      normalized round trip.
- [x] Strokes hold for the author's chosen time then fade; nothing persists beyond
      `stroke_hold_max_ms` + fade. Playwright: a stroke is still there after 2.5 s of its 3 s hold
      and gone, pixels included, soon after the fade. The hold travels with the stroke and is capped
      by the instance (unit tests); strokes are kept in memory only.
- [x] Sharer disabling drawing hides pen controls for others within 500 ms; flood test (synthetic
      2,000 points/s) is throttled without UI jank. Playwright: the sharer's switch ends the other
      person's draw mode and removes their pen within 500 ms, and turning it back on returns it. The
      flood is a unit test (2,000 points a second for a second keep under 500 points); the receiver
      limits (240 points/s per author, 8 strokes at once, 40 per share, 4,000 points per stroke)
      bound what a canvas draws, and the canvas redraws only while something is on it.
- [x] Measured stroke latency < 100 ms on LAN (instrumented test). Playwright measures from the
      pen going down on one page to the point appearing on the sharer's canvas: 45–65 ms on one PC
      (points leave in 25 ms batches).

### Phase 5 acceptance

- [x] A teammate on a different ISP joins via the Funnel URL from a browser, hears voice and sees a
      1080p60 share. Done with five friends on 2026-10-16. All of them went through the relay then,
      which cost quota and frame rate; [ADR 0004](adr/0004-server-reflexive-candidates.md) is the
      fix, to be confirmed on the next evening.
- [x] With the host's router UPnP disabled and TURN configured, a client forced to `relay` connects
      (relay test passes) and the admin panel shows the relayed session. The owner's router answers
      none of PCP, NAT-PMP or UPnP; with a Cloudflare TURN key the relay test passed and the
      friends' sessions counted as relayed.
- [x] Switching exposure provider in the admin panel works without data loss; quick-tunnel URL
      change is surfaced. Checked in the browser: local → Cloudflare quick tunnel → local from
      Settings → Connectivity; the status and the public URL followed live over the gateway,
      admins get a notice with the new address, and the hub warns when a quick tunnel's address
      replaces an earlier one. Unit tests cover the service with a fake provider.
- [x] TURN budget guard stops issuing credentials past the budget (test with a tiny budget): the
      relay service's tests spend a 1 GB budget, check that no relay is issued, then override it.
- [ ] Release artifacts run on a clean Windows machine by double-click → wizard opens. The release
      workflow builds the archives on a tag; a Windows zip built locally holds `hub.exe`,
      `livekit-server.exe` and a README, and the hub opens the setup page in the browser on first
      run. A clean machine is manual script step 1.

### Phase 4 acceptance

- [x] 1440p60 Text share of a code editor is crisp in Focus on a second machine (stats show
      2560×1440 and ~60 fps on a healthy LAN); Motion mode on a UE5 viewport keeps ≥ 55 fps.
      Measured with `task bench:share` on one PC (sharer and viewer in separate browsers): a
      synthetic code editor captured at ~50 fps reached the viewer's focused tile at 2560×1440 and
      48 fps with the default codec (AV1), whether typing or scrolling every frame; a synthetic
      3D viewport kept 49 of 51 fps with VP9. The test browser could not draw the synthetic screen
      faster than ~50 fps, so a true 60 fps source, a real editor, a real UE5 viewport and a second
      machine are steps 3 and 7 of the manual script.
- [x] With 3 simultaneous shares and 5 viewers in Grid, viewers receive low layers (stats) and the
      sharer's dynacast pauses unused layers. `task bench:share`: eight people in one call, three
      sharing 1080p30; every viewer received all three shares at 960×540 (the low layer), and
      every sharer's 1920×1080 layer was paused at 0 kbps. When one viewer focused a share in a
      large window, that sharer's full layer resumed for that viewer only.
- [x] Focus/unfocus/fullscreen/grid transitions never show a black or frozen tile; share ending
      moves focus correctly. Each share has one `<video>` that moves between tiles instead of
      being recreated. The Playwright suite tags the element, goes grid → focus → fullscreen →
      back, and checks it is the same element and still advancing each time; ending a share with
      the browser's stop button returns the viewer's stage to the grid. The focus fallback (next
      share, then the grid) is unit-tested.
- [x] Decision on default codec per mode recorded in an ADR with measured CPU/bitrate numbers:
      [ADR 0002](adr/0002-screen-share-codecs.md). Automatic uses AV1 for text and VP9 for motion,
      every codec in two simulcast layers with a VP8 backup.

### Phase 3 acceptance

- [x] Three users on LAN in two channels: audio isolated per channel; switching channels < 1 s;
      sidebar in other guilds shows occupants live. Each voice channel is its own LiveKit room
      (`vc_<channelId>`), so audio cannot cross channels. Switching measured 56–66 ms from the
      Join call to LiveKit's confirmation in the integration test, and well under a second in the
      browser (the Playwright test switches and checks the other browser). Voice states go to
      every member of the guild whatever page they are on; the Playwright suite checks the other
      browser within 1 s. A three-person call with real people is step 4 of the manual script.
- [x] Deafen mutes all remote mics and own mic; undeafen restores the previous mute; states
      persist across reload and apply on join; others see icons update live. Reducer tests cover
      the exact semantics; the Playwright suite mutes, deafens (remote audio goes silent),
      undeafens back to muted, reloads, rejoins muted and watches the icons in the other browser.
- [x] Server mute cannot be bypassed by the client: integration test against a real
      `livekit-server` with a Go participant. After the mute, LiveKit removes the published
      microphone, refuses a new one and the client's permissions lack it; tokens minted later
      carry no microphone grant, and every confirmed connection gets the permissions re-applied
      (a token from before the mute cannot sneak one in).
- [x] Killing a client process clears its voice state within `departure_timeout`; restarting the
      hub restores voice states from LiveKit. Integration test: a participant process is killed
      and its state clears after ~21 s (LiveKit gives up on the connection after ~20 s). A fresh
      voice service rebuilds the call from `ListParticipants`, and clients that reconnect to a
      restarted media server are adopted from their webhooks (unit tests).
- [x] Joining from a second device moves voice there and the first shows the "another device"
      notice. Integration test (the first participant is dropped with `DUPLICATE_IDENTITY`) and
      Playwright (a second tab joins; the first shows the notice and leaves the call).
- [x] Device unplug falls back to default without dropping the call. On `devicechange` the client
      switches a vanished microphone to the default inside the same LiveKit room, and LiveKit does
      the same for the output; the choice is unit-tested. Unplugging real hardware is step 3 of the
      manual script.

### Phase 2 acceptance

- [x] Two browsers: creating a guild, channel and invite, joining, renaming, reordering, role
      changes and presence changes appear on the other browser in < 500 ms without refresh. The
      Playwright suite (`e2e/tests/guilds.spec.ts`) drives an admin and a member in two browser
      contexts and asserts each change on the other side with a 500 ms timeout: join, channel
      create, rename, drag-and-drop reorder, promotion to admin and a Do Not Disturb status.
- [x] Killing the network for 30 s then restoring resumes the gateway without a full reload
      (verified by seq replay test); after 3 min it re-identifies cleanly. Replay is tested in Go
      (`transport/gateway`: drop without a close frame, events while away, Resume with the last
      seq, replay in order, no presence flap) and in the browser: the suite cuts the gateway
      WebSocket, renames a channel meanwhile, restores the network and checks that the page
      resumed (first frame Resume, not Identify), applied the rename and was not reloaded. The
      expired path is tested on both sides: the hub answers InvalidSession after the resume
      window or a gap in the replay buffer, and the client then identifies on the same
      connection. The E2E outage lasts a few seconds rather than 30 s; the resume window is 120 s.
- [x] `max_guilds_owned_per_user` enforced (creation and ownership transfer); non-members cannot
      see a guild in any API or gateway event. Every guild, channel, invite and moderation method
      answers `NotFound` to outsiders (service tests and an HTTP test with a banned user); the
      gateway delivers guild events only to sessions subscribed to that guild's topics (topic
      tests: join subscribes, kick unsubscribes after the removal event).
- [x] Kick/ban remove the guild from the target's UI immediately; a banned user cannot accept new
      invites. Covered in Go (HTTP + gateway) and in the browser: the kicked member's rail loses
      the server and a notice explains why; after a ban, the invite page says the user is banned.
- [x] Policy matrix fully unit-tested (owner/admin/member × every action), including target
      rules (owner, self, admin on admin) and the members-can-invite setting.

### Phase 1 acceptance

- [x] Fresh data dir → wizard → admin created → login works; setup unreachable afterwards and from
      non-localhost. Covered by the Playwright suite (wizard, sign-in, `/setup` redirect after
      completion), the HTTP flow tests in `internal/app`, and service and handler tests for remote
      clients (setup RPCs answer `PermissionDenied not_local`, the `/setup` page answers 404).
- [x] Register in `invite_only` requires a valid instance invite; `open`/`closed` behave
      correctly. Service tests cover each mode; the Playwright suite registers a newcomer through a
      real invite link after a refused attempt without one.
- [x] With SMTP: verification and reset emails are sent, tokens are single-use and expire, and a
      reset revokes every session. Without SMTP, verification is off and admins can create reset
      links (RPC plus a UI under Settings → Instance → Users). Covered by service tests, and the
      SMTP adapter is tested against an in-process SMTP server. **Not yet tried with a real provider
      (Gmail app password):** that is step 6 of the Phase 1 manual script.
- [x] Unknown user and wrong password return the identical error, checked in service tests, over
      HTTP and in the browser. Rate limits answer `ResourceExhausted` with a retry delay, checked in
      service tests and over HTTP.
- [x] Unit tests cover every auth and account service path, and the repository tests run on real
      SQLite.

### Phase 0 acceptance

- [x] `task dev` runs hub + Vite with proxy; browser shows the app shell and a live health check.
      Verified in a browser: status goes online → unreachable when the hub stops → online again when
      it restarts, without a reload.
- [x] `task build` produces a single hub binary that serves the built web app. Verified by running
      `bin/hub` from an unrelated folder, and by the Playwright suite (`task e2e`), which runs against
      that binary.
- [x] CI green on a PR. [PR #1](https://github.com/tornellihenrique/dislackso-rebirth/pull/1): all
      five jobs passed on the first run (lint + gen freshness, Go tests on ubuntu and windows, web
      typecheck + tests, build + Playwright smoke).

## Done

### Phase 7 — Profile cards, moving people, sidebar width

- **Profile card everywhere:** clicking someone in the member list or in a voice channel, or
  oneself in the user panel, opens the same card (`UserPopover`): profile, member since and role,
  their voice channel, and while they share Watch / Stop watching / Join and watch / Open with a
  live preview (copied from the stage when the share shows there). One's own card sets the status
  and links to Edit profile and Settings.
- **Moderators drag people between voice channels** in the sidebar (native drag and drop onto a
  voice channel row, which moves them through the hub); "Move to" no longer shows empty when
  there is no other voice channel.
- **Resizable channel sidebar**, 200–420 px as §15.1 says, saved on the device; double-click resets
  it, arrow keys move it.
- **Crop with previews:** one crop dialog for avatars and server icons (which had none), with live
  previews at the sizes they show, drag, wheel zoom and 1–4× zoom.
- **Fixed:** dragging a server icon or a channel reloaded the app (see decisions); an avatar
  picture stretched its voice row and turned the speaking ring into a capsule; the tray icon was
  empty.

### Phase 7 — Live share options, speaking as heard, tray

- **Quality and sound change during a share** from the share button's menu, without restarting it:
  `retuneShare` applies the preset's capture constraints and each layer's caps to every sender in
  place (the publication, its SID and drawings stay); a full reconnect applies the current preset
  again. The menu has a "Share sound" checkbox, the same saved setting as the desktop picker's.
- **Share sound:** the sound is captured whenever the picker gives it (the desktop app on Windows
  always asks for the loopback) and published muted when off. In the browser, Chrome's own picker
  still decides whether there is any sound; without it the checkbox explains why it is off.
- **Speaking ring from the audio heard here:** levels read every 50 ms from what this device plays,
  after per-user volume, mute and deafen, and from the microphone signal sent; the media server's
  delayed active-speaker reports are no longer used. The 400 ms hangover stays.
- **Own preview pauses** while the app is in the background or another window has the focus; the
  tile says so.
- **Desktop tray:** closing the window hides it in the tray and the call goes on; the tray opens it
  again, checks for updates or quits.
- **Check for updates on demand:** Settings → Desktop app (listed only in the app) shows the version
  and checks now, through the bridge's optional `updates`; the tray's item does the same and says
  what it found in a dialog. Background throttling is off for the instance window, so a hidden call keeps its
  heartbeats.

### Phase 7 — Direct address with a free DuckDNS name

- **Direct exposure keeps a DuckDNS name pointed at the hub** (`adapter/ddns`, behind
  `ports.DynamicDNS`): before the certificate the provider reports the `dns` stage, finds the
  public IPv4 with STUN and updates DuckDNS; then it checks every 5 minutes and updates on change.
  A failed update is the `dynamic_dns_failed` problem and the provider retries with the usual
  backoff.
- **Settings:** `ExposureSettings.dynamic_dns` (none or DuckDNS) and the write-only
  `dns_token`, stored in the secret store like the tunnel token. With DuckDNS the domain must be
  `<name>.duckdns.org` and the token is required.
- **Connectivity page:** "Keep the domain pointed here" and the DuckDNS token under "Public
  address with a domain"; HOSTING.md has the steps.
- **Direct mode serves its port while the first certificate is requested.** It used to ask for the
  certificate before serving, so Let's Encrypt's TLS-ALPN handshake on port 443 was never answered
  and the first certificate could not be issued. The request also asks for the ECDSA certificate
  browsers use, so the first visitor does not cause a second issuance.
- **A Let's Encrypt rate limit shows at once** as `certificate_rate_limited` with its message, and
  the hub waits for the Retry-After it gave (an hour at most) before the next try. The ACME client
  used to sleep through the Retry-After inside the request, so the page stayed on "Getting an
  HTTPS certificate…" while every try timed out after 5 minutes.
- **A failed certificate check shows Let's Encrypt's reason** (`certificate_check_failed`): a
  transport around the ACME client keeps the error of an invalid authorization, which autocert
  replaces with "no viable challenge type found". The next check waits 15 minutes, under Let's
  Encrypt's 5 failed checks per name per hour.
- **With DuckDNS the certificate comes over Let's Encrypt's DNS check** (`direct/dns01.go`, on
  `x/crypto/acme`): the TXT record goes out through DuckDNS (`ports.DNSChallenge`), so the
  check needs no inbound connection. The owner's provider lets Brazilian networks reach port 443
  but drops Let's Encrypt's checks from abroad ("Timeout during connect"). Each request runs on
  Let's Encrypt's staging server first, and the hub waits until public DNS (DNS-over-HTTPS)
  shows the record before asking for the check; a record that never shows is
  `dns_record_missing`, given back unchecked. The certificate is saved in `acme/` and renewed
  when a third of its lifetime is left. Without dynamic DNS the TLS-ALPN check stays.
- **An own domain can follow DuckDNS:** `ExposureSettings.dynamic_dns_name` is the DuckDNS name the
  hub keeps pointed when the direct domain is a CNAME to it; empty keeps the domain itself.

### Phase 7 — After the first evening with the desktop app

- **Laser color:** the laser packet carries its color (`Laser.color_rgba`); it used to borrow the
  author's last stroke color.
- **Picker quality:** the desktop picker has the quality preset next to the sound checkbox; the
  share reads the preset once the user picked, so a change there applies to that share.
- **Closing settings** (and server settings, within the same server) goes back to the page they
  were opened from instead of home.
- **Strokes fade per author:** an author's strokes on a share hold while they keep drawing and fade
  together after their last line.

### Phase 7 — Desktop shell (part 2: installers and updates)

- **Installers** (electron-builder 26.17, `task desktop:dist`): Windows NSIS one-click for the
  current user without admin rights, macOS dmg (x64 and arm64, ad-hoc signed), Linux AppImage and
  deb. The package holds only electron-vite's bundle and `package.json`. The icon is the product
  mark (`apps/web/public/favicon.svg`) rendered at 1024 px.
- **Updates** (electron-updater 6.8): a packaged app checks the latest published release at start
  and every 6 hours. Windows and the AppImage download it in the background and ask to restart
  (otherwise it installs on quit); macOS, unsigned, opens the download page instead.
- **Release workflow:** on a tag, a job per OS builds the installers with the tag as the version;
  they go into the draft release with the updater's `latest*.yml`, the blockmaps and checksums.
- **Tests:** `E2E_DESKTOP_EXE` runs the desktop e2e against a packaged app; it passed on the
  unpacked Windows build, with the updater on.

### Phase 7 — Desktop shell (part 1)

- **`apps/desktop`** (Electron 44.8, electron-vite 6): a launcher that takes a server's address
  (https, or http on this computer only), checks `/healthz` and remembers up to ten servers, each in
  its own `persist:` partition. The server opens in a sandboxed window with context isolation that
  keeps navigation on the server (other links open in the system browser, no webviews) and grants
  only media, display capture, notifications, fullscreen, clipboard writes and speaker selection,
  only to the server's origin. Menu: switch server, edit, view.
- **Bridge** (`packages/desktop-bridge`, `window.desktop`, version 1): the preload asks the main
  process whether the page is the window's own server before exposing anything; every call is
  checked in the main process (sender, frame, origin) and its arguments validated.
- **Screen picker:** the app answers `getDisplayMedia` with the web app's own dialog: Screens and
  Windows tabs with thumbnails refreshed every 2 s, and on Windows "Share your computer's sound"
  (loopback).
- **Drawing overlay:** sharing a whole screen opens a transparent window over that display: on top
  of everything, left out of the capture, click-through except its toolbar. It draws every stroke
  on the share with `@dislackso/annotation` from the packets the web app relays. Its toolbar: draw
  on the screen (the overlay then takes the mouse until Esc; the pen goes to the web app, which
  draws it as the sharer's stroke), clear everyone's strokes, stop or allow others drawing, stop
  sharing. The overlay closes when the share ends.
- **Tests.** Unit: the bridge's validation, server addresses and partitions, security rules, shell
  strings, the picker dialog, the drawing controller with a fake overlay. Playwright with Electron
  (`task e2e:desktop`, Windows): the launcher, sharing through the picker, the overlay's window
  properties, a viewer's stroke on the real screen and not in the video, the sharer drawing on the
  overlay, and stopping the share from it.

### Phase 6 — Drawing on shares

- **Contract.** `proto/dislackso/annotation/v1`: `AnnotationPacket` with stroke begin, points, end,
  undo, clear, laser and the share's drawing policy (ARCHITECTURE §11.3). `Ready.instance` now
  carries `stroke_hold_max_ms`.
- **`packages/annotation`** (framework-agnostic): geometry (content rect of an `object-fit:
  contain` video, normalized points packed as uint16 pairs), styles and colors, timing (hold, 0.8 s
  fade, name tags, laser trail), receiver limits, packet codec, the `AnnotationEngine` (this
  client's strokes as packets, everyone else's from packets, the policy, undo and clear, pruning)
  and the `CanvasRenderer` (midpoint-smoothed pen, highlighter, arrow, glowing laser, name tags).
  26 unit tests.
- **Web.** The voice session sends and receives LiveKit data packets (topic `annot`) and reports
  who joins; `DrawingController` (`app/drawing.ts`) follows the call's shares, the channel's
  `drawing_allowed` and this device's share policy, batches points every 25 ms and sends the
  policy again to whoever joins. Share tiles show strokes on a canvas over the video; the pen button
  (or D on the focused share) enters draw mode with a toolbar: pen, highlighter, arrow, laser, eight
  colors and a custom one, three sizes, undo, clear mine, snapshot, and on one's own share clear all
  and "stop others from drawing". Esc, right-click or the X leave it. Snapshot copies the frame
  with its strokes as PNG (or saves it). Settings → Drawing: color, tool, size, hold time (capped by
  the instance), name tags, and whether others may draw on one's shares.
- **Tests.** Unit: the package, the drawing controller (policy to joiners, channel off, what goes
  reliable). Playwright: the four acceptance checks above, inside the share spec.

### Phase 5 — Hosting & connectivity

- **Ingresses.** Each way into the hub is its own listener tagged with an ingress
  (`transport/ingress`): local or not, HTTPS or not, and whom to trust for the client address.
  Setup, LiveKit webhooks and host options accept only the hub's own listener; cookies are
  `Secure` and HSTS is sent whenever the client used HTTPS, even through a tunnel
  ([ADR 0003](adr/0003-hosting-ingress-and-reachability.md)).
- **Exposure.** Tailscale Funnel through an embedded tsnet node (login link in the admin panel,
  guidance for HTTPS and Funnel permissions, log upload off), Cloudflare quick and named tunnels
  (cloudflared downloaded on first use, pinned and checksummed, named tunnels on
  `localhost:7301`), direct with Let's Encrypt (autocert, TLS-ALPN), LAN with a self-signed
  certificate, and local. Settings live in the DB; the exposure service restarts the provider on
  save, retries failures with backoff, keeps the public URL in step and pushes status changes to
  instance admins.
- **Relays (TURN).** Cloudflare Realtime TURN (credentials minted per join, port-53 URLs dropped),
  coturn REST secrets and static credentials; the relay service is the voice ICE provider. Voice
  clients report their connection path and relayed bytes every 30 s; the hub counts them per
  month, projects the month, and stops issuing relays past the budget unless overridden.
- **Doctor.** STUN from the media port before LiveKit binds it classifies the NAT (no internet
  turns LiveKit's external IP lookup off instead of crash-looping), IPv6 over STUN, port forwarding
  over PCP, NAT-PMP or UPnP kept up and removed on stop, and a rating with advice. Runs at start
  and on demand; `hub doctor` runs the network checks from a console.
- **Host options.** Keep awake (on by default), run at login and the Windows firewall rule
  (elevation prompt), changeable only from the hosting machine. The hub opens the setup page in the
  browser on first run, and a Windows console it was double-clicked into waits for Enter after an
  error.
- **Connectivity page** (Settings → Connectivity, instance admins): public address chooser and
  live status, the self-test, relay settings with usage and a browser relay test, and the host
  options. Setup finishes there. The public URL on the Instance page is read-only while a provider
  sets it.
- **Accounts.** Email change (password, then a link to the new address; immediate without SMTP)
  and account deletion (password; owners and the last instance admin are refused; the account is
  anonymized and hidden, leaving every server). The instance-admin flag now reaches a user's
  sessions live (`ME_UPDATE`).
- **Release.** `.github/workflows/release.yml` builds, on a `v*` tag, archives for Windows, macOS
  and Linux on amd64 and arm64 with the pinned livekit-server, plus `checksums.txt`, into a draft
  release. `task release:archive GOOS=… GOARCH=…` builds one locally.
- **Docs.** [HOSTING.md](HOSTING.md): run, set up, Tailscale or Cloudflare, relay, Gmail, host
  options, troubleshooting, ports and bandwidth.
- **Tests.** Go: ingress client IP and locality, exposure service with a fake provider, relay
  issuing, usage and budget, NAT classification and rating, port forwarding against a fake PCP and
  NAT-PMP router, the doctor loop, host options, TURN providers against a fake Cloudflare, the
  cloudflared installer, email change and deletion (service and SQLite). Web: transport meter,
  telemetry reports, connectivity forms, dispatcher. Playwright: setup lands on Connectivity, email
  change without SMTP (22 tests in all).

- **After the first test with a friend:** the hub gives LiveKit its public IPv4 (voice never came
  up on a host with broken IPv6), `task build` puts livekit-server next to the hub, images over
  10 MB are refused before uploading (the hub's early refusal could reach the browser as a dropped
  connection), the share tile's fullscreen button toggles, and dragging a server icon no longer
  drags the link itself: dropping it used to make the browser open it, reloading the app and
  ending the call. A page-wide guard keeps any other dropped link or file from doing the same.
  (That was not the cause: the reload still happens, see Known issues.)
- **After the first evening with the team:** LiveKit gets a node IP only when the router keeps the
  media port's number; otherwise it gathers server-reflexive candidates over STUN, so callers
  behind ordinary routers connect directly instead of through the relay
  ([ADR 0004](adr/0004-server-reflexive-candidates.md)), and the self-test rates such a router
  Good. "Optimize for" and the "Share audio" toggle are gone: the preset's frame rate sets the
  tuning (60 fps keeps the frame rate with VP9, 30 fps keeps text sharp with AV1), and the
  browser's own picker decides about audio.

### Phase 4 — Screen share & stage

- **Hub.** The instance's share limits (`max_shares_per_channel`, `max_share_preset`) reach
  clients in `Ready.instance`. A share the media server reports past the channel's limit is
  stopped: the hub takes the screen share sources out of the participant's permissions for a
  moment, which makes LiveKit unpublish it, and gives them back for a later share. The streaming
  flag still comes only from LiveKit's track webhooks. Unit tests, and the integration test against
  a real LiveKit (a second share is removed, the permission comes back, a share within the limit
  stays).
- **Sharing.** `core/media/screen-share.ts` captures with `getDisplayMedia` (the preset's size
  as ideal, frame rate capped, constraints applied again after capture, system and tab audio with
  `restrictOwnAudio` where the browser has it) and publishes the video in two simulcast layers
  with the preset's bitrate, the content hint and degradation preference of the chosen
  optimization, and a VP8 backup codec; the audio as a stereo 128 kbps track of its own. Presets,
  caps and encodings are a pure module (`share-presets.ts`) with tests. The browser's own "Stop
  sharing" ends the share; a share the server stops shows why.
- **Watching.** `VoiceSession` reports everyone's shares as LiveKit delivers them, plays share
  audio with a volume per sharer (0–200 %, kept on the device) apart from their voice, and stops
  or resumes receiving a share. Deafening leaves share audio alone unless "Deafening also mutes
  the audio of screen shares" is on.
- **Stage.** Grid (the §10.3 algorithm, a pure function with tests), Focus with a collapsible
  filmstrip, Fullscreen on the stage with controls and cursor hiding after 3 s, keyboard (G, F,
  ←/→, Ctrl+Shift+S), auto-focus of the first share someone else starts and fallback when the
  focused share ends (a reducer with tests), "show only screens", and the floating control bar
  (mic, deafen, share split button with quality, optimization and audio, fullscreen,
  disconnect). Share tiles: LIVE badge, name, hover toolbar (focus, fullscreen, stats, mute
  stream, stop watching or stop sharing), context menu with the stream volume; user tiles: avatar
  on the user's color, voice icons, speaking ring, connection quality on hover.
- **No black tiles.** Each share has one `<video>` owned by the voice controller
  (`ShareViews`); tiles mount it through a callback ref, so a tile moving between grid, focus,
  filmstrip and fullscreen keeps its picture.
- **Stats.** A stats overlay per share tile (resolution and layer, fps, codec and decoder,
  bitrate, loss, jitter, round trip, path), and on the sharer's own preview each sent layer and
  whether dynacast paused it. The voice panel's quality icon opens connection details (ping,
  loss, direct or relayed path, codec).
- **Settings → Screen share.** Quality (capped by the instance), optimization, share audio,
  auto-focus, own preview, deafen-mutes-streams, stats overlay, and the codec (Automatic, VP9,
  VP8, H.264, AV1).
- **Measurements.** `task bench:share` runs a hub and synthetic screens (a code editor and a
  3D-viewport-like scene) in Chromium, and writes the codec and layer tables of
  [ADR 0002](adr/0002-screen-share-codecs.md).
- **Tests.** Go: share limit (service and real LiveKit). Web: presets and caps, stats parsing,
  share views, grid layout, tiles, stage reducer, the controller's share flow (caps, limit,
  stop reasons). Playwright (`share.spec.ts`, Chromium's fake capture): the share shows LIVE to
  everyone and takes the viewer's stage, the same video keeps playing through grid, focus and
  fullscreen, the stats overlay, stop and resume watching, and the browser's stop button ending
  the share with focus returning to the grid (21 tests in all).

### Phase 3 — Voice

- **Contract.** `VoiceService` (Join, Leave, MoveMember, SetServerMute, SetServerDeafen,
  DisconnectMember), `VoiceState` (with a `connected` flag that stays false until LiveKit
  confirms), voice states in `GuildState`, gateway events `VOICE_STATE_UPDATE` and
  `VOICE_FORCED`, the `UpdateVoiceSelfState` frame, and audit actions for voice moderation.
- **Voice service.** The hub owns voice state in memory: one connection per user on the whole
  instance; a join mints a LiveKit token (identity = user id, room = `vc_<channelId>`, the voice
  session id as participant metadata) and removes the user from any other room. LiveKit webhooks
  confirm joins, end connections and set the streaming flag; unconfirmed joins expire after 30 s.
  Participants the hub does not know are adopted when they are members (a restarted hub) and
  removed otherwise; an older connection of the user that comes back is removed. User limits
  (owners and admins exempt, moved members exempt), membership checks, a join rate limit, and
  cleanup when a member, channel or guild goes away.
- **Moderation.** Server mute and deafen are stored per member (migration `00003_voice.sql`) and
  enforced through LiveKit permissions; move (the member's client rejoins the target channel) and
  disconnect. All three are owner/admin actions under the same target rules as kick, and land in
  the audit log.
- **LiveKit sidecar.** `internal/sidecar` supervises processes: health probe, restart with
  backoff (1–30 s), log capture to `logs/livekit.log`, a Windows job object (process group and
  parent-death signal elsewhere) so LiveKit never outlives the hub. The hub writes
  `livekit.yaml` on every start (signaling on localhost only, UDP 7882 mux, TCP 7881, external
  IP via STUN, webhooks to the hub) and passes the API keys, generated once and kept in
  `secrets.yaml`, through the environment. `/readyz` reports the media server.
- **HTTP.** `/lk/rtc…` reverse-proxies LiveKit signaling (WebSocket included) so it shares the
  instance address; the rest of LiveKit (admin API, debug pages) stays unreachable. The webhook
  endpoint answers only on loopback and verifies the signature. Tokens never reach the logs.
- **ICE.** `IceServerProvider` with `none` (wired) and `static` implementations.
- **Web core.** `VoiceSession` wraps the LiveKit room: microphone publishing (Opus, DTX, RED,
  the instance's bitrate cap), deafen by disabling remote microphone forwarding, reconnect states,
  quality, active speakers and autoplay unblocking. `MicProcessor` runs the microphone through
  RNNoise (optional, loaded on demand), input volume and a voice gate (automatic or manual
  sensitivity, 400 ms hangover) or push-to-talk (focused window, released on blur).
  `RemoteAudio` plays everyone through `<audio>` elements kept outside any view, with per-user
  volume up to 200 % (Web Audio above 100 %), local mute and output device. Synthesized sounds for
  join, leave, mute, deafen and disconnect. `livekit-client` loads with the first join.
- **Web app.** `VoiceController` joins through the hub, keeps the hub told about mute and deafen
  (again after every gateway reconnect), and reacts when the hub ends, moves or replaces the call.
  Mute and deafen work in and out of calls and persist on the device; Ctrl+Shift+M and
  Ctrl+Shift+D toggle them.
- **UI.** Occupants under each voice channel in every guild (speaking ring, mute, deafen and
  server icons, LIVE badge, dimmed while connecting), clicking a voice channel joins it, a voice
  panel above the user panel (state, channel, quality, disconnect, enable-audio button), mic and
  headphone toggles in the user panel (red when a moderator muted you), a voice channel page with
  a join screen and a tile per participant, voice entries in member context menus (volume slider,
  local mute, server mute and deafen, move to, disconnect), and Settings → Voice & audio (devices,
  volumes, mic test with meter and playback, input mode and push-to-talk key, sensitivity, noise
  suppression, echo cancellation, gain control, sounds). Everything in en and pt-BR.
- **Tests.** Go: voice service on fakes (joins, limits, switching, confirmation, adoption, stale
  sessions, self state, server mute and deafen, move, disconnect, sweep, media restart,
  reconcile, cleanup), sidecar supervisor with a helper process, LiveKit adapter (token grants,
  Twirp calls and their admin grants, webhook signatures, config), proxy and webhook handlers,
  config, and `task test:int` against a real LiveKit with Go participants. Web: speaking hangover,
  voice gate, push-to-talk, devices, voice stores and reducers, dispatcher, gateway self state and
  the controller's decisions. Playwright: five voice tests with two browsers and a synthetic
  microphone (17 tests in all).

### Phase 2 — Guilds, channels, realtime shell, basic moderation

- **Contract.** `GuildService`, `ChannelService`, `InviteService` (guild invites, preview and
  accept) and `ModerationService` (kick, ban, unban, bans, audit log); the gateway protocol in
  `gateway.proto` (frames, close codes as an enum, the event catalog for this phase); presence,
  file references and avatars on `User`.
- **Domain.** Guilds, members, channels (category, text, voice; text names normalized to
  `release-notes` style), guild invites (8-character codes from an unambiguous alphabet, fixed
  expiry and use options), bans, audit entries, files, presence, and the whole permission matrix
  in `domain.Policy` (`Can` plus target rules for moderation, roles and ownership).
- **Data.** Migration `00002_guilds.sql` (files, guilds, members, bans, invites, channels,
  audit log; deleting a guild cascades). Repositories with tests on real SQLite. Transactions
  gained after-commit hooks.
- **Events.** An in-process bus. Services publish through a publisher that waits for the
  transaction to commit, so receivers never see rolled-back changes and a snapshot read after an
  event always includes it.
- **Services.** `guild` (create with default channels named in the creator's language, update,
  delete with name confirmation, leave, transfer ownership, roles, members), `channel` (create,
  update, delete — a category's channels move out —, full reorder), guild invites in `invite`
  (create, list, revoke, preview signed out, accept, and registration with a guild invite),
  `moderation`, `presence` (in memory: online while a gateway session lives; idle when every
  connection reports inactivity; dnd and invisible from the preference), `snapshot` (Ready), and
  `file` (avatar and icon uploads). `account` now sets avatars and publishes profile and
  status changes. Every moderation and settings change writes the audit log and tells admins.
- **Gateway** (`/gateway`). Hello → Identify or Resume → Ready or Resumed with replay; 25 s
  heartbeats with a dead-connection watchdog and a session re-check on each beat; 2,000-event,
  120 s replay buffers; topics per user, guild and guild admins, updated as people join, leave
  and change roles; close codes; Origin check; per-connection frame rate limit; slow readers are
  dropped and resume; shutdown closes everyone with "hub restarting".
- **Files.** `POST /api/uploads` (multipart, size cap, rate limit), `GET /files/{id}/{name}`
  (session required; sandbox CSP, nosniff, immutable caching), and the invite landing icon at
  `/api/invites/{code}/icon`. Images are decoded and re-encoded on the hub (a square of at
  most 256 px, centered or the part picked in the avatar cropper; JPEG, PNG when transparent),
  which drops metadata and anything hidden after the image; decompression bombs are refused
  before decoding. Animated GIFs stay animated (at most 160 px, 300 frames, 2000 × 2000):
  frames are decoded one at a time, composed as browsers do and re-quantized, and files
  `image/gif` refuses but browsers show (frames past the logical screen, a broken last frame)
  still work. Blobs are stored by
  SHA-256 under `files/`.
- **Web core.** `GatewayClient` (resume first, backoff 0.5–15 s with jitter, retry at once when
  the browser comes back online or visible, zombie detection by missed heartbeat acks, idle after
  10 min without input) and `GatewayDispatcher`, the only writer of entity stores from push
  events; RPC results go through the same reducers, which keep the newest copy by `updated_at`.
  Stores for entities, presence, realtime status, notices, appearance and local UI preferences.
- **Web UI.** The §15.1 frame: rail (server icons with pill and tooltip, drag to reorder on this
  device, add server), channel sidebar (server menu, collapsible categories, hover actions,
  context menus, drag-and-drop reorder for admins), channel header with member list toggle,
  placeholders for voice and text, member list (owner, admins, online, offline) with profile
  popout and moderation menu, connection banner, notices. Dialogs to create or join a server,
  create channels and categories, edit channels, and invite people. Invite landing page. Server
  settings: overview, channels, members, invites, bans, audit log, danger zone. User settings:
  avatar with cropping, appearance (theme and density, applied before the first paint).
- **Tests.** Go: policy matrix, validators, repositories, every new service on fakes, adapters
  (imaging, blob store, codes), gateway over real WebSockets (identify, ordering, resume,
  refusals, close codes, heartbeat timeout, shutdown, origin, topics), and HTTP flows in
  `internal/app` (guild invite registration, gateway delivery between two users, kick, ban,
  uploads). Web: gateway client with a fake socket and fake timers, dispatcher, reducers, channel
  tree and drag math, member groups, permissions mirror, upload errors, and the invite landing
  page and member list. E2E: `guilds.spec.ts` as above.

### Phase 1 — Accounts & instance setup

- **Data.** SQLite through `modernc.org/sqlite` (WAL, foreign keys, `busy_timeout`, one writer
  connection with `_txlock=immediate`, a read-only pool). Goose migrations are embedded in the
  binary and run on start (`00001_accounts.sql`: users, sessions, email_tokens, instance_settings,
  all `STRICT`). sqlc generates the queries (`hub/sqlc.yaml`). `TxRunner` carries the
  transaction in the context, so repositories join it without services seeing SQL types.
  Snowflake IDs (epoch 2026-01-01, 41/10/12 bits; monotonic when the clock steps back).
- **Ports and adapters.** argon2id (m=64 MiB, t=3, p=2, PHC strings, at most 4 hashes at once),
  random 256-bit tokens with only SHA-256 hashes stored, an in-memory token-bucket rate limiter,
  `secrets.yaml` (0600, atomic writes), and mail through an SMTP adapter (go-mail: STARTTLS, TLS or
  none) or the development mail log.
- **Services.** `settings` (cached; validated patches), `auth` (register for every registration
  mode, login, sliding 30-day sessions refreshed every 10 minutes, logout, email verification,
  resend, password reset, admin reset links, setup state), `account` (me, display name, status
  preference, change password, sessions), `invite` (single-use instance invites; registration
  claims them through an `InviteClaimer` that guild invites can implement later), `instance`
  (admin settings, test email, users, disable/enable, promote/demote, reset links), `setup`
  (localhost-only, one-time token, admin plus settings in one transaction) and `notify`
  (localized text and HTML emails in en and pt-BR).
- **Transport.** RPC middleware rejects cross-site `Origin`s, records the client (IP, user agent,
  best locale), resolves the `dsx_session` cookie (HttpOnly, SameSite=Lax, Secure over TLS),
  renews it when the session slides, and clears it once the session is gone. Domain errors become
  Connect codes with typed details (`ErrorReason`, `FieldViolations`, `RateLimited`); unexpected
  errors are logged and answered with a bare `Internal`. The `/setup` page is served only to the
  host machine and only while setup is pending. New config keys: `mail.dev_log` and
  `setup.token`. The hub prints the setup link on start, and warns when email is off.
- **Web.** Session store (Zustand vanilla store, pure reducer), route guards (setup pending →
  `/setup-required`, signed out → `/login?redirect=`, unverified → `/verify-pending`,
  non-admin → no instance settings), and a Connect interceptor that treats `Unauthenticated`
  outside `AuthService` as a lost session. Screens: setup notice and 4-step wizard (instance,
  admin, email with test send, connectivity placeholder); login, register (invite from the link),
  check your inbox, email confirmation, forgot password, reset password; settings (My account
  with password change, Profile with display name and status, Sessions with revoke); instance
  admin (general, email, invites, users). There is a user panel in the shell, and error reasons
  map to localized text.
- **Tooling.** sqlc and goose pinned in `tools/manifest.json`; `task gen` runs buf and sqlc,
  `task gen:check` also runs `sqlc diff`, `task db:migration -- <name>` creates a migration, and
  `task dev` starts the hub with the mail log and a fixed dev setup token.
- **Tests.** Go: domain validators, SQLite repositories, every adapter (including SMTP against an
  in-process server), every service on in-memory fakes, RPC middleware and error mapping, the
  `/setup` gate, and full HTTP flows in `internal/app` (setup, invite, registration, identical
  login errors, rate limit, logout, cross-origin refusal). Web: session reducer, error decoding and
  text, SMTP form mapping, setup step routing, safe redirects, user agents, and the login and
  register pages (Testing Library). E2E: the serial Playwright flow described in the acceptance
  section above.

### Phase 0 — Foundation

- **Monorepo.** pnpm workspaces (`apps/web`, `packages/config`, `packages/contracts`, `e2e`) with
  one version catalog in `pnpm-workspace.yaml`; Go modules `hub/` and `tools/`; `Taskfile.yml`
  (`setup`, `tools`, `dev`, `gen`, `gen:check`, `lint`, `fmt`, `typecheck`, `test`, `test:int`,
  `check`, `build`, `e2e`); `.editorconfig`; `.gitattributes` forcing LF everywhere (the dev
  machine has `core.autocrlf=true`); golangci-lint v2 with gofumpt + goimports formatters; ESLint
  10 with typescript-eslint `strictTypeChecked`; Prettier with the Tailwind plugin.
- **Pinned tools.** `tools/manifest.json` lists buf, golangci-lint and livekit-server with a
  SHA-256 per platform. `task tools` (stdlib-only Go program `tools/cmd/fetchtools`) downloads,
  verifies and installs them into `.tools/bin`, skipping up-to-date ones. LiveKit publishes no
  macOS binaries, so on macOS it builds `livekit-server` from the pinned, hashed source tarball.
- **Contract.** `proto/` with Buf v2 config (STANDARD lint, FILE breaking rules for later);
  `dislackso.v1.HealthService/Check` and the empty `ClientFrame`/`ServerFrame` envelopes in
  `dislackso/gateway/v1/gateway.proto`. `task gen` writes `hub/gen` (protobuf-go + connect-go) and
  `packages/contracts/src/gen` (protobuf-es). `task gen:check` (`tools/cmd/gencheck`) regenerates
  into a scratch dir and diffs against the working tree, so it also works before committing.
- **Hub.** `hub [run] | version | help`. Config precedence: defaults < `config.yaml` (unknown keys
  rejected) < `DSX_*` env < flags; per-OS data dir; `slog` text or JSON; graceful shutdown on
  SIGINT/SIGTERM. Routes: `/rpc/*` (Connect, `Connect-Protocol-Version` required, 1 MiB request
  cap), `/healthz` (liveness), `/readyz` (runs `health.Probe`s; none registered yet), and the
  embedded SPA (index fallback for client routes, immutable caching for `/assets`, 404 for missing
  assets, dotfiles and reserved API prefixes, strict CSP). Security headers on every response;
  access log at debug level. Layering: `ports.Clock` → `adapter/clock`, `service/health`,
  `transport/{rpc,web}`, composition in `internal/app`.
- **Web.** Vite 8, React 19, TypeScript 6 strict (`noUncheckedIndexedAccess`,
  `exactOptionalPropertyTypes`, `verbatimModuleSyntax`), Tailwind v4 with design tokens only (the
  default palette, radii and fonts are cleared) and Dark/Onyx/Light themes from §15.3, TanStack
  Router + Query, i18next with typed keys in `en` and `pt-BR` (parity is checked by the compiler
  and a test). `app/` is the composition root; `core/` (api, i18n, format) cannot import React
  (ESLint rule). The app shell has a live hub health check that polls every 5 s (sidebar badge and
  a detail card). Fonts (Inter, JetBrains Mono) are self-hosted.
- **Tests.** Go: config, data dir, health service, RPC handler over HTTP, SPA/health routes,
  middleware, app start/shutdown, CLI, tool fetcher, gen check. TS: i18n, API mapping (in-memory
  Connect transport), relative time, health status, health card (Testing Library). E2E: Playwright
  smoke suite against the built binary (shell, live health, deep-link reload, pt-BR, no console or
  CSP errors).
- **CI.** `.github/workflows/ci.yml`: lint + gen freshness (ubuntu), Go tests (ubuntu, windows),
  web typecheck + tests (ubuntu), build + Playwright smoke + binary artifact (ubuntu).
- **Docs.** README, this file, [ADR 0001](adr/0001-record-architecture.md).

## Decisions / ADRs

- [ADR 0001](adr/0001-record-architecture.md): the ledger in `ARCHITECTURE.md` §2 is the
  architecture record; changes and new decisions of the same weight get ADRs.
- No ledger decision was changed in Phase 0.
- **Go module path** `github.com/tornellihenrique/dislackso-rebirth/hub`, following the git remote.
- **Version pinning:** the newest release that is at least a day old (pnpm 12's default
  supply-chain gate). Releases published on the day of setup were swapped for the previous one:
  React 19.3.0 (19.3.1 only touches Server Components, which we do not use), i18next 26.4.4,
  lucide-react 1.56.0, Playwright 1.64.0, buf 1.73.0, livekit-server 1.13.9, connect-go v2.0.0.
- **TypeScript 6.0.3, not 7.0.2.** typescript-eslint 8.71 supports TypeScript `<6.1`, and the
  type-aware strict lint needs it. Move to 7 when typescript-eslint supports it.
- **connect-go v2** (`connectrpc.com/connect/v2`, released 2026-10-07). It is a new Go API, not a
  new protocol, so Connect-ES 2.2 works unchanged. Handlers take and return plain messages. Choosing
  it now avoids a v1 → v2 migration later.
- **Generators run through `go tool`** (tool directives in `hub/go.mod`), so protoc-gen-go and
  protoc-gen-connect-go always match the runtime libraries. protoc-gen-es comes from
  `packages/contracts`. Generated TS is imported by path (`@dislackso/contracts/gen/...`), without a
  barrel file.
- **`transport/http` is named `transport/web`:** golangci's revive flags package names that shadow
  the standard library.
- **CSRF baseline already on:** RPC requests without `Connect-Protocol-Version` are rejected (§5.4).
- **int64 on the wire → `bigint` in TS;** `core/api` mappers convert Unix-ms timestamps to `number`.
- **`/readyz` lists only the names of failing probes;** probe errors can reveal host paths.
- **Hub CLI and logs are English-only** (operator-facing). All UI strings go through i18n.
- **Layout:** `packages/config` holds the tsconfig and ESLint presets; the Prettier config is a
  single root file. `apps/desktop` and `packages/annotation` will be created in Phases 7 and 6.
- **Data-dir resolution** (listed in Phase 1) was done now because `config.yaml` lives there.

### Phase 1

- No ledger decision was changed.
- **Validation lives in `domain`, not protovalidate (§5.4).** The rules include more than shape
  checks (common passwords, uniqueness, registration mode), and violations travel as typed
  `FieldViolations` details that the client localizes. Adding protovalidate later would mean a BSR
  dependency and a connect-go v2 interceptor for little gain right now; revisit if client-side reuse
  becomes worth it.
- **The SMTP password is kept in `secrets.yaml`, not in the database,** so nightly backups and the
  planned Litestream replicas (§17.7) never carry it. The API treats it as write-only
  (`password_set` tells the UI whether one is stored).
- **Emails link to the configured public URL, never to request headers:** a forged `Host` would
  otherwise send reset tokens elsewhere. The wizard pre-fills the URL from the browser; exposure
  providers will set it in Phase 5.
- **Instance invites** are single-use `email_tokens` (purpose `instance_invite`, `user_id` = the
  admin who created it), 7 days by default, at most 30 days. Only a hash is stored, so the link is
  shown once.
- **Lifetimes:** verification links last 24 hours, emailed reset links 30 minutes, and
  admin-created reset links 24 hours (they are handed over by hand).
- **Without SMTP, accounts stay unverified but are not gated.** If email is configured later,
  existing unverified accounts are asked to confirm their address. The setup admin starts verified.
- **Rate limits** (token buckets): login 10 per IP and 5 per account, then 1 per 6 s and 1 per
  minute; register 5 per IP, then 1 per 10 min; email-token use 10 per IP; resend verification 2
  per user, then 1 per 2 min; reset requests 5 per IP and 3 per email; password change 5 per user;
  setup checks 10 per IP.
- **Admins cannot disable or demote themselves,** so an instance never loses its last admin by
  accident.
- **Deferred:** email change and account deletion (`RequestEmailChange`, `DeleteAccount`).
  Deletion needs guild-ownership rules from Phase 2. Opening the browser on first run (§4.1) moves
  to Phase 5's host experience. All three were done in Phase 5.
- **Settings are a full-screen route** (`/settings/...`) rather than a modal over the app, so deep
  links and the back button work. Phase 2 can render it as a modal over the shell.

### Phase 2

- No ledger decision was changed.
- **`UpdatePresence` carries only `idle`.** The status preference and custom status go through
  `AccountService.SetStatusPreference`, so they persist across restarts and reach every device;
  the frame only reports inactivity (ARCHITECTURE §8.4 updated).
- **Revoked sessions end their gateway connection at the next heartbeat** (≤ 25 s) instead of
  through a `SESSION_REVOKED` event; RPCs fail at once. The re-check also catches expiry and
  disabled accounts without extra events.
- **No missed events between snapshot and subscription:** events are published after commit, and
  Identify subscribes before it reads the snapshot (§8.1 updated).
- **Presence survives a dropped connection for the resume window** (120 s), so a network blip
  does not flap; closing the tab or signing out ends it at once.
- **Deleting a guild deletes it for good** (channels, members, invites, bans, audit log), rather
  than a soft delete: nothing references a deleted guild yet.
- **Avatars and guild icons are visible to every signed-in user,** like profile pictures; the
  invite landing page gets the icon through the invite code. Uploaded images are stored at most
  256 px; `?size=` thumbnails are not needed yet.
- **Guild invites also register newcomers** while registration is invite-only, and the account
  joins the guild. Members see and revoke their own invites; owners and admins see all.
- **Banning works on former members too,** and ownership transfer respects the new owner's limit
  of owned guilds.
- **CSP style nonce.** Radix injects a `<style>` element to lock scrolling behind dialogs and
  menus; the hub now issues a fresh nonce per page (in the CSP header and a meta tag) instead of
  allowing `'unsafe-inline'` (§5.4 updated).
- **Local preferences** (member list shown, collapsed categories, last channel per server, rail
  order, theme, density) live in versioned `localStorage` entries; `public/appearance.js`
  applies the theme before the app loads, so a light theme never flashes dark.
- **Settings stay routes:** user settings at `/settings/...` and server settings at
  `/guilds/{id}/settings/...`, both full screen with Escape to close.
- **New dependencies:** radix-ui 1.8.1, @dnd-kit/core 6.3.1 + sortable 10.0.0, react-easy-crop
  6.2.4, get-nonce 1.0.1 (already used by Radix; set with the page nonce), coder/websocket 1.8.15,
  golang.org/x/image 0.47.0.

### Phase 3

- No ledger decision was changed.
- **`MediaProvider` as built** (§5.2 updated): permissions change in one call
  (`SetPermissions`), and there is no separate track mute, because LiveKit unpublishes a track
  whose source is no longer allowed. Webhook parsing lives in the adapter, behind an interface of
  the HTTP handler.
- **Server mute and deafen belong to the member,** not the connection: leaving and joining again
  does not lift them. LiveKit enforces them; the hub re-applies them whenever a connection is
  confirmed, since tokens refresh on their own and one minted before a mute could reconnect.
- **The voice session id travels as participant metadata.** Participant webhooks match on it;
  track webhooks carry only the participant SID, so the hub remembers the SID of each confirmed
  connection.
- **Move keeps the session id** and the member's client joins the target itself: LiveKit's
  `MoveParticipant` exists only on LiveKit Cloud. Moved members may enter a full channel.
- **One connection per user across rooms is the hub's job:** LiveKit drops duplicate identities
  only inside one room, so a join removes the user from the room they were in. In the same room
  LiveKit replaces the old connection itself (removing it from the hub could hit the new one).
- **LiveKit keys** are generated once, kept in `secrets.yaml`, and reach the sidecar through
  `LIVEKIT_KEYS`; `livekit.yaml` never holds the secret.
- **`livekit.external_ip` defaults to true** (§4.3). `task dev`, the e2e suite and the
  integration tests turn it off: LiveKit refuses to start when STUN is unreachable.
- **CSP allows `'wasm-unsafe-eval'`** so RNNoise can compile its WebAssembly; JavaScript eval stays
  blocked (§5.4 updated).
- **Mute, deafen and voice preferences are per device** (`localStorage`), which matches devices,
  volumes and processing; syncing mute and deafen across devices can come with settings sync.
  Unmuting while deafened also undeafens.
- **`livekit-client` loads on the first join** (a 526 kB chunk), keeping the first page load at the
  Phase 2 size.
- **Sounds are synthesized tones** (Web Audio), so there are no audio assets to license.
- **The `static` TURN provider has no settings yet;** choosing and configuring TURN belongs to the
  Phase 5 wizard, with the Cloudflare provider.
- **Voice shortcuts are fixed** (Ctrl+Shift+M, Ctrl+Shift+D) until the Keybinds page.
- **New dependencies:** github.com/livekit/protocol v1.52.1 (hub), github.com/livekit/server-sdk-go
  v2.19.0 (integration tests only), livekit-client 2.22.4, @sapphi-red/web-noise-suppressor 0.4.1.

### Phase 7

- No ledger decision was changed (D10: a thin Electron shell loading the instance). ARCHITECTURE
  §16.3–16.5 updated with what was built.
- **Electron 44.8.0 and electron-vite 6.0.0**, the newest releases at least a day old. Electron
  downloads its binary on first use, not at install, so pnpm's blocked install scripts do not get
  in the way.
- **The overlay sends the sharer's pen to the web app instead of packets.** The web app's engine
  stays the one author of this user's strokes (packets, undo, clear); the overlay only renders and
  takes input.
- **The overlay's toolbar has no color picker;** it draws with the color and size from Settings →
  Drawing, which the web app applies.
- **Sandboxed preloads are built as one CommonJS file each,** and workspace packages are bundled
  into the main process: preloads can load no other file, and workspace packages ship TypeScript.
- **The desktop e2e runs locally, not in CI:** it shares and draws on the real screen, and capture
  protection is a Windows (and macOS) feature.
- **Strokes fade per author and share,** at the team's request: lines drawn one after another stay
  until their author stops drawing, then fade together after the hold.
- **Settings return to the last app page** (home or a channel), remembered from the router's
  resolved locations; sign-in and settings pages are never return targets.
- **electron-builder 26.17.0 and electron-updater 6.8.10.** The updater is bundled into the main
  process with everything else, so packaging never walks pnpm's `node_modules`.
  electron-winstaller's install script (Squirrel.Windows, unused by NSIS) is denied in
  `pnpm-workspace.yaml` (`allowBuilds`).
- **One-click per-user NSIS:** no admin rights and no choices; the app starts when it is installed.
- **macOS builds are ad-hoc signed** (`identity: '-'`, hardened runtime off): electron-builder
  does not fall back to it, and Apple Silicon refuses unsigned apps.
- **The app's version is the release tag** (`extraMetadata.version`); local builds use
  `git describe`.
- **The click that ends a dnd-kit drag is cancelled from window** (`drag-click.ts`): dnd-kit only
  stops its propagation, so React never saw it while the browser still followed the dragged
  item's link, a full page load that dropped the call. Reproduced in the browser and fixed for the
  rail and the channel list.
- **The speaking ring wraps the avatar in a flex box:** as an inline box it sat on the text
  baseline, which a picture puts at its bottom edge, so the line grew and the ring followed it.
- **The tray icon is the app's PNG bundled with the main process** (`?asset`), scaled to 16 px
  with a 32 px copy; the executable's own icon came back empty.
- **Live quality changes keep the codec.** Another codec needs a new publication, which viewers
  would see as a new share (focus, drawings and the share's SID would reset).
- **Speaking ring uses local levels only,** as the spec already allowed ("active-speaker + local
  level"): the server's reports arrive late and keep showing people this device does not hear.
- **The tray reuses the executable's icon** (`app.getFileIcon`), so no loose image ships with the
  app.
- **Direct mode with DuckDNS** for hosts with a public IP, so the team's address no longer
  depends on Tailscale's relays. No ledger change: D7 already lists direct as an alternative and
  DuckDNS is free. It is a dynamic DNS port, so other providers fit without touching the direct
  provider. `duckdns.org` is on the Public Suffix List, so Let's Encrypt limits count per name,
  and autocert caches the certificate in `acme/` so restarts do not request a new one.
- **The public IPv4 comes from the STUN servers the doctor already uses,** not from DuckDNS
  guessing the caller's address, so an IPv6 request can never point the name at the wrong
  family.

### Phase 6

- No ledger decision was changed. ARCHITECTURE §11 updated with what was built.
- **The share's drawing policy is sent again to whoever joins** (only to them), since late joiners
  get no history: a packet on change alone would leave them thinking drawing is allowed.
- **The laser shows while the button is held,** like the pen, and repeats its place every second
  while held still; receivers drop a laser after 2.5 s without news.
- **A stroke longer than 3,000 points continues as a new one,** so its end (which carries every
  point, to repair lost batches) fits in one reliable packet.
- **The sender keeps under the receivers' limit:** points closer than 0.0008 or sooner than 5 ms
  after the previous one are dropped before sending.
- **Lines are at least 2 px wide** on screen: at the spec's widths a medium line in a small tile was
  under 2 px.
- **Draw mode hides the tile's hover toolbar;** snapshot moved into the drawing toolbar.
- **The drawing tests live in the share spec:** a spec of their own made the suite pass the
  per-account login limit (5 a minute).

### Phase 5

- No ledger decision was changed; [ADR 0003](adr/0003-hosting-ingress-and-reachability.md)
  records the hosting design. ARCHITECTURE §4, §5, §7.3, §8.4 and §12 updated.
- **Setup continues on the Connectivity page** instead of adding exposure, relay and test steps
  to the wizard: the same page serves later changes, and the self-test needs the media server up.
- **The self-test result is kept in memory,** refreshed at every start and on demand, rather than
  stored.
- **Relay budgets count decimal gigabytes**, as Cloudflare bills them.
- **The hub gives LiveKit the public IPv4 it found** (`rtc.node_ip`, with LiveKit's own lookup
  off). LiveKit's STUN lookup prefers IPv6; on a host whose IPv6 route is broken (a second router
  advertising itself as an IPv6 gateway it cannot serve) it took over a minute, past the
  supervisor's 30 s health limit, so the media server was restarted forever and voice never came
  up.
- **Funnel shows running only once its address works.** A new Funnel name reached public DNS about
  eight minutes after it was enabled, and the certificate used to be issued during the first
  visitor's handshake (about 40 s). The provider now fetches the certificate first, then checks
  the address from outside (public DNS-over-HTTPS at Google, then Cloudflare, since the local
  resolver may keep a cached "no such name"), reporting each stage; after 15 minutes it serves
  anyway, marked unverified.
- **Rating "Excellent" with IPv6 alone** follows §4.4, though friends without IPv6 then depend on
  the host's NAT; HOSTING.md recommends the free relay anyway.
- **Account deletion anonymizes the row** (`deleted_at` was already in the schema) so audit
  entries and bans keep their references; lookups skip deleted accounts, so history shows them as
  "Someone".
- **Run at login repeats the command line** the hub was started with (Windows `Run` key, macOS
  LaunchAgent, Linux XDG autostart), not a service install.
- **livekit-server for macOS is built from the pinned source on a Mac:** its CPU stats need cgo,
  so the release jobs for macOS run on a macOS runner, and `task tools` keeps cgo on for native
  source builds.
- **LiveKit gathers its own outside ports** ([ADR 0004](adr/0004-server-reflexive-candidates.md),
  ledger D8 reworded): a pinned `node_ip` made LiveKit advertise 7882, which a router that picks
  its own port numbers never forwards, so every caller from outside used the relay.
- **The preset's frame rate sets the share tuning** (ARCHITECTURE §9.5, ADR 0002 update), at the
  team's request: a preset already names a resolution and a frame rate.
- **New dependencies:** tailscale.com v1.104.1 (tsnet), github.com/huin/goupnp v1.3.0, github.com/pion/stun/v3 v3.1.7,
  golang.org/x/crypto/acme/autocert (already a module), actions/download-artifact v8.0.2 (release).

### Phase 4

- No ledger decision was changed.
- **Codecs and layers:** see [ADR 0002](adr/0002-screen-share-codecs.md). Every codec is sent as
  two simulcast layers (full and half size), not single-stream SVC: LiveKit's client sends VP9
  screen shares as one spatial layer when asked for SVC, and dynacast cannot pause SVC layers.
  ARCHITECTURE §9.5 updated.
- **The share limit is the hub's to enforce; the quality caps are the client's.** LiveKit can
  refuse a track but cannot cap a publisher's resolution or bitrate.
- **The default preset is 1080p60, optimized for text.** 1440p60 is one click away in the share
  menu; 1080p60 keeps a sharer's upload and the host's egress at about 4 Mbps per viewer.
- **Quality, optimization and audio apply to the next share.** The share menu disables them
  while sharing, rather than restarting the capture behind the user's back.
- **Stage tiles are sorted by name** (shares first, then people), not by who speaks or joined first.
  ARCHITECTURE §10.2 updated.
- **Fullscreen is the stage element** (§10.2), so tiles never move into a portal; menus and
  tooltips render inside the stage while it is fullscreen. Together with `ShareViews` this keeps
  the CLAUDE.md rule about fullscreen tiles: the video element is never recreated.
- **This device's own share is never focused by itself;** auto-focus is for others' shares.
- **Share and stage preferences are per device** (`localStorage` `dsx.share`), like the voice
  preferences.
- **No new dependencies.**

## Known issues

- **The release binaries are unsigned:** Windows SmartScreen and macOS Gatekeeper warn on first
  run (HOSTING.md says how to proceed). The macOS release jobs have not run yet; the first tag
  will show whether livekit-server builds there as expected.
- **Run at login on Windows opens a console window** at sign-in (the hub is a console program);
  a tray host comes with the desktop shell (Phase 7).
- **The Cloudflare TURN API is called on every voice join;** credentials are not cached yet.
- **A public URL left by a stopped provider stays** until the admin edits it or another provider
  runs, so invite links may point at a dead quick tunnel.
- **Advice and rating text in `hub doctor` are English only** (like the rest of the CLI).
- Expired sessions and used or expired email tokens are never purged yet; a periodic cleanup job
  is still to come.
- Replaced avatars and icons stay on disk, like expired sessions; the cleanup job will cover
  both.
- The dev-build frame inspector (§8.1) is not built yet.
- Rail badges (unread, mentions) wait for text chat.
- No `LICENSE` file yet: the license is the owner's choice.
- The web bundle is ~977 kB minified (296 kB gzip) plus a 535 kB LiveKit chunk loaded with the
  first join, so Vite prints its 500 kB chunk warning on build. Route-level code splitting
  (settings, server settings) is the next easy win.
- Go's `Server.Shutdown` waits up to 5 s for connections a client opened but never used (for
  example a browser preconnect), so stopping the hub can take a few seconds.
- **Browsers allow the microphone and screen sharing only on HTTPS or localhost,** so people on
  other machines need an exposure provider (any of them gives HTTPS; LAN uses a self-signed
  certificate).
- A client that dies without leaving stays in voice for ~20 s, until LiveKit gives up on it.
- The sidecar stops with the hub, so restarting the hub also empties LiveKit's rooms. Clients
  reconnect on their own within LiveKit's retry window (~40 s) and the hub adopts them; after a
  longer outage people rejoin by hand.
- Disabling or deleting an account ends its sessions but not a voice connection already up
  (LiveKit keeps refreshing its token); the next join is refused.
- A host whose router keeps port numbers and that loses internet after starting keeps LiveKit's
  last external address until the hub restarts.
- **Dragging a server icon in the rail still reloads the page on the owner's PC** (Chrome, real
  mouse), which ends the call. Playwright does not reproduce it, also during a call with slow
  drags; the app has no reload of its own, so the next step is a DevTools capture (Console and
  Network with "Preserve log") of one drop.
- **The owner's host has IPv6 addresses but no IPv6 route out** (2026-10-17: the gateway answers,
  nothing past it does). The second router in the way (an ONT used as a switch) is the likely
  cause; the self-test reports IPv6 as missing, which is right.
- livekit-client logs some of its own races and data channel closings as console errors; the
  e2e error tracker ignores lines tagged with a LiveKit room.
- **The codec numbers come from one PC** (Core Ultra 9, RTX 5070 laptop) with synthetic screens
  and software encoders: Chromium for testing exposes no hardware encoder, so H.264 ran on
  OpenH264. Real machines, real content and hardware H.264 are steps of the manual script; the
  defaults may change after the team uses them (Phase 5).
- Pop-out (Picture-in-Picture) comes in Phase 11.
- **A sharer sees drawings on the real screen only in the desktop app,** and only when sharing a
  whole screen; sharing one window shows them on the tile, until the window-share overlay.
- **The installers are unsigned:** Windows SmartScreen asks to confirm the first run ("More info"
  → "Run anyway"), and macOS needs right-click → Open. macOS builds cannot update themselves and
  show a download link; the Linux deb updates by installing the new one (the AppImage updates
  itself). The macOS and Linux installers are built by the release workflow but not yet tried.
- **The computer's sound shared from the desktop app includes the call's voices** on Windows; the
  exclusion is still to come.
- **The desktop app is tested on Windows only;** macOS and Linux builds are not made yet.
- **Someone who joins mid-stroke sees strokes only from then on** (no history, by design); a
  stroke whose end was lost ends by itself 5 s after its last point.
- Without `restrictOwnAudio` (older Chromium, other browsers) shared system audio can carry the
  call's voices back; the sharer gets a warning.
- The stage shortcuts (G, F, arrows, Ctrl+Shift+S) are fixed until the Keybinds page.
- **The tray has no mute or deafen yet,** only open and quit; on macOS its icon is the colored app
  icon, not a template image.
- **In the browser, sound must be picked when the share starts:** Chrome's picker decides whether
  the share captures any; "Share sound" can then turn it off and on, but cannot add it later.
- **Direct mode without dynamic DNS needs port 443 reachable from abroad** for Let's Encrypt's
  TLS-ALPN check; with DuckDNS the DNS check needs no open port for the certificate.
- **The public address may not open from the hub machine itself** when the router has no NAT
  loopback; the host uses `http://localhost:7300` (the desktop app accepts it).
- **DuckDNS gets an IPv4 record only,** which every client can reach.

## Manual test script

### Profile cards, moving people, sidebar width

1. Click someone in the member list, in a voice channel, and yourself in the bottom-left panel: the
   same card opens; yours changes the status and links to Edit profile and Settings.
2. While a friend shares: their card shows Watch / Stop watching (with a preview) or Join and
   watch, and Open takes you to the stage with the share in focus.
3. As an admin, drag someone in voice onto another voice channel: they move there.
4. Drag the sidebar's right edge wider and narrower; reload: the width stays.
5. Change your avatar and a server icon: drag, zoom with the wheel, and check the previews.
6. Drag a server icon in the rail while in a call: the order changes and the call stays.

### Live share options and tray

Prerequisites: the hub and the desktop app built from this branch, a friend in a call.

1. Share your screen. Open the arrow next to the share button: pick another quality. Your friend's
   view changes within a second or two without the share stopping; their drawings stay.
2. In the same menu, untick and tick "Share sound" while playing a video: your friend hears it go
   away and come back.
3. Talk, then mute a friend locally (right-click → Mute): their ring stops lighting while they talk.
   Deafen: nobody's ring lights.
4. Click into another window: your own preview says it is paused; click back and it plays.
5. Desktop app: close the window during a call. The call goes on; the tray icon opens the window
   again; right-click → Quit Dislackso ends it.
6. Settings → Desktop app shows the version; Check for updates says what it found (in the installed
   app; a run from the project says updates work in the installed app only). The tray's Check for
   updates answers in a dialog.

### Direct address with DuckDNS

Prerequisites: the router sends port 443 to the hub machine (DMZ or forward), a DuckDNS name and
token.

1. Settings → Connectivity → Public address: choose "Public address with a domain", set "Keep the
   domain pointed here" to DuckDNS, enter the name and token, Save and restart. Allow the hub in
   the Windows Firewall prompt (Private and Public).
2. The status shows "Pointing the domain…", then "Getting an HTTPS certificate…", then reachable
   with `https://<name>.duckdns.org`. On duckdns.org the name shows your public IP.
3. From a phone on mobile data, open the address and sign in; join a call.
4. Restart the hub: it comes back without a new certificate (the `acme/` folder in the data
   folder keeps it).
5. If the status says Let's Encrypt is not issuing a certificate right now, earlier failed tries
   hit its limit: leave the hub running; it tries again at the time in the message.

### Phase 7 (part 1)

Prerequisites: the repo updated on the Windows PC, and a friend in a call.

1. **Open.** Run `task desktop`. The first time it asks for the server's address: paste the Funnel
   address and press Connect. Sign in. Next time it opens the server by itself (Server → Switch
   server to change it).
2. **Share.** Join a voice channel and share: the app shows its own picker with your screens and
   windows. Pick a screen, tick "Share your computer's sound" if you want it, press Share.
3. **Overlay.** A small toolbar appears at the top of the shared screen. Your friend draws on your
   screen in their browser: the strokes appear on your real monitor, with their name, and fade.
   Their view of your screen does not show each stroke twice.
4. **Draw yourself.** Press the pen on the toolbar: a blue frame shows you are drawing. Draw on your
   screen; your friend sees it. Esc stops.
5. **Toolbar.** Try "Clear everyone's strokes", the lock (stops others drawing; press again to
   allow), and drag the toolbar by its dots. Stop sharing from it: the toolbar goes away.
6. **Window share.** Share one window instead: no overlay appears (drawings show on the tile only).

### Phase 6

Prerequisites: the hub updated, two or three people in a voice channel, at least two of them
sharing a screen.

1. **Draw.** Hover a friend's screen and press the pen (or press D on the focused one). Draw: the
   line follows the pen with your name next to it; everyone, the sharer included, sees it on the
   same spot of the content, also in a small grid tile and in fullscreen.
2. **Fade.** Lift the pen: the line stays about 3 s, then fades. Settings → Drawing → "Strokes stay
   for" 8 seconds makes it last longer for everyone.
3. **Tools.** Try the highlighter (wide, see-through), the arrow (drag start to end) and the laser
   (a glowing dot with a short trail while you hold the button). Change color and size; undo your
   last stroke; clear yours.
4. **Two at once.** Two people draw on two different screens at the same time; both see both.
5. **Owner.** On your own screen (grid view, your tile's pen), press "Stop others from drawing":
   the others' pen disappears at once and their strokes go. Press it again to allow. "Clear
   everyone's strokes" removes every stroke on your screen.
6. **Channel.** As an admin, turn drawing off in the voice channel's settings: the pen disappears
   for everyone in it.
7. **Snapshot.** In draw mode press the camera: the frame with its strokes is copied (paste it in a
   chat) or saved.

### Phase 5

Prerequisites: a Tailscale account (HOSTING.md step 3), optionally a Cloudflare account for the
relay, and one or two people on other networks with Chrome or Edge.

1. **Release.** Push a tag such as `v0.5.0` after merging; the Release workflow drafts a release
   with six archives and `checksums.txt`. On a Windows PC that never ran the hub, download the
   Windows zip, extract it and double-click `hub.exe` (SmartScreen: More info → Run anyway). The
   browser opens the setup page.
2. **Setup.** Finish the wizard; the Connectivity page opens and says only this machine can open
   the hub.
3. **Funnel.** Choose Tailscale Funnel, Save and restart, click Sign in to Tailscale and approve.
   The status turns green with `https://….ts.net`, and Settings → Instance shows it as the public
   URL (read-only). If it says HTTPS or Funnel is off, follow the message and wait a minute.
4. **Invite.** Settings → Instance → Create invite link; send it to a teammate on another network.
   They register and land in the app.
5. **Voice.** Both join a voice channel: you hear each other. The voice panel's signal icon shows
   Direct or Relayed.
6. **Share.** Share a 1080p60 window; the teammate's stats overlay (Ctrl+Shift+S) shows 1920×1080
   near 60 fps.
7. **Relay.** Create a Cloudflare TURN key (HOSTING.md step 4), save it, press Test the relay: "The
   relay works". With the teammate in a call, the usage box counts relayed connections when their
   network needs the relay. Run the reachability test: with no port forwarding it suggests UPnP.
8. **Budget.** Set the monthly budget to 1 GB on a test instance and have someone relay a few
   minutes of a share: past 1 GB new joins get no relay and the panel says the budget is spent;
   ticking "Keep the relay on" brings it back.
9. **Switching.** Switch to Cloudflare quick tunnel: the address changes to `….trycloudflare.com`,
   admins see a notice, and nothing else is lost (servers, members, settings). Switch back to
   Funnel.
10. **Host options.** On the hosting PC: untick and tick Keep awake, tick Start the hub when I
   sign in and sign out and in again, and press Allow voice through Windows Firewall (Windows asks
   to confirm). From another PC the page says these change only on the hosting machine.
11. **Doctor.** In a console in the hub's folder, `hub doctor` prints the router type, addresses,
   port forwarding and the rating.
12. **Email (Gmail).** Set up Gmail (HOSTING.md step 5), send a test email; Settings → My account →
   Change email sends a link to the new address, and opening it changes the address.
13. **Delete.** A test account deletes itself from My account (password asked): it leaves every
   server and cannot sign in again; its username can be registered anew.
14. **Language.** In pt-BR every Connectivity text, notice and email is in Portuguese.

### Phase 4

Prerequisites: `task setup`. As in Phase 3, a "second person" is a second browser profile on the
same PC, or a second PC whose Chrome treats your LAN address as secure (Phase 3, step 4).

1. `task check` is green, `task e2e` passes 21 Playwright tests, and `task test:int` passes.
   Optional: `task bench:share` (about 10 minutes) writes a fresh table to `e2e/bench-results/`.
2. `task dev`; profiles A and B join the same voice channel.
3. **Share.** In A, open the arrow next to the share button in the bottom bar, pick 1440p · 60 fps
   and Text and detail, then press the share button and pick a window with code. B: the sidebar
   shows LIVE next to A, and B's stage switches to A's screen by itself. Text is sharp; scroll
   in the editor and it stays smooth. Press Ctrl+Shift+S in B: Resolution shows 2560×1440 (on a
   1440p or larger screen), the codec AV1, and the frame rate stays near 60 while the editor
   moves.
4. **Layouts.** In B: G for the grid (A's screen gets small and the stats say "low layer"), double
   click it for focus, F for fullscreen (controls and cursor hide after 3 s, Esc leaves), ← and →
   to move the focus, the arrow above the filmstrip to hide it. The picture never goes black or
   freezes while switching.
5. **Stop watching.** Hover A's screen in B and press the eye: a "You stopped watching" panel;
   Watch brings it back.
6. **Audio.** A shares a browser tab playing sound with "Share tab audio" ticked. B hears it;
   right-click the screen for Stream volume; deafening B keeps the stream audio unless Settings →
   Screen share → "Deafening also mutes…" is on.
7. **Motion.** A picks Motion (games, 3D) and shares a game or a 3D viewport: the stats stay at
   55–60 fps.
8. **Codec.** Settings → Screen share → Video codec: H.264, share again; the stats show H264 (and
   on a machine with a hardware encoder, an encoder name other than OpenH264).
9. **Stop.** A ends the share with Chrome's own "Stop sharing" bar: the share ends for both, B's
   stage goes back to the grid, and the stream sound plays.
10. **Limits.** Server settings → Channels → the voice channel → Highest screen share quality:
   720p · 30 fps. A's share menu then offers only 720p.
11. **Connection.** Click the signal icon in the voice panel: ping, packet loss, Direct (local
   network) · udp, OPUS.
12. **Language.** In pt-BR every stage control, menu, stats label and setting is in Portuguese.

### Phase 3

Prerequisites: `task setup` (installs `livekit-server` into `.tools/bin`). Browsers only allow
the microphone on HTTPS or localhost, so until Phase 5 a "second person" is a second browser
profile on the same PC, or a second PC whose Chrome treats your LAN address as secure (step 4).

1. `task check` is green, `task e2e` passes 17 Playwright tests, and `task test:int` passes (it
   runs a real LiveKit; the killed-client case takes ~20 s).
2. `task dev`: the hub log shows `sidecar started` and `sidecar ready` for livekit, and
   `.dev/data/livekit.yaml` exists without any secret in it.
3. **Settings → Voice & audio.** Pick your microphone and press Let's check: the meter moves as you
   talk and turns green past the yellow threshold mark; Play my voice back (with headphones) lets
   you hear yourself. Switch noise suppression to Strong (RNNoise): typing gets quiet, your voice
   does not. Choose Push to talk, record a key, and hold it: the meter turns green only while you
   hold it. While in a call later, unplug a USB mic: the call goes on with the default mic.
4. **A call.** In profile A click the voice channel: you hear the join sound, "Voice connected"
   shows above your name, and you appear under the channel. In profile B (or another PC with
   Chrome started with `--unsafely-treat-insecure-origin-as-secure=http://<host LAN IP>:5173` and
   the hub's `http.addr` on the LAN) join the same channel: you hear each other, and the speaking
   ring lights up. With a third person in a second voice channel, the two calls do not hear
   each other, and everyone sees who is where in the sidebar.
5. **Mute and deafen.** Toggle the mic and headphones in the user panel (or Ctrl+Shift+M and
   Ctrl+Shift+D): the other profile sees the icons at once. Mute, then deafen, then undeafen: you
   are still muted. Reload: the state stays, and joining again starts muted.
6. **Volume.** Right-click the other person: set their volume to 150 % and back, and Mute for me.
7. **Moderation** (as owner or admin, on a member): Server mute shows a red icon for everyone and
   their mic button turns red; leaving and joining again keeps it. Move to another voice channel:
   they land there with a notice. Disconnect from voice: they drop with a notice. Each shows in
   Server settings → Audit log.
8. **Another device.** With profile A in a call, join from a second tab of the same profile: the
   first tab shows "Voice connected on another device" and leaves the call.
9. **Hub restart.** Restart `task dev` while in a call: the panel shows Reconnecting, then
   Connected again within a minute, and the sidebar is right on both sides.
10. **Language.** In pt-BR every voice screen, menu and notice is in Portuguese.

### Phase 2

Prerequisites: `task setup`. Use two browser profiles (or one normal and one private window) so
two accounts can be signed in at once.

1. `task check` is green, and `task e2e` passes 12 Playwright tests.
2. **Setup.** Delete `.dev/data`, run `task dev`, finish setup at
   <http://localhost:5173/setup?token=dev-setup-token>. Home shows "Welcome" with Create and Join.
3. **Create a server.** Use the **+** in the rail (or Create a server): pick an icon and a name.
   You land in #general; the sidebar has a text and a voice category; the member list shows you as
   owner with a crown, online.
4. **Invite and join.** Server menu (click the server name) → Invite people. Change the expiry and
   uses and see the link change. Open the link in the other profile: the landing page shows the
   server, who invited you and member counts. Choose Create an account (the code is filled in);
   confirm the email with the link from the hub log. The new member appears in the first
   profile's member list right away, as online.
5. **Live changes.** In the first profile: create a channel (server menu → Create channel), rename
   it (right-click → Edit channel), drag it to another category, make the other user admin
   (right-click them in the member list). Each change appears in the second profile at once. In
   the second profile set Do Not Disturb with a custom status (click your name in the user panel,
   or Settings → Profile): the first profile shows the red dot and the status.
6. **Reconnect.** In the second profile, stop the hub for ~10 s (Ctrl+C on `task dev:hub` in a
   separate terminal) and start it again: a "Reconnecting…" banner appears after 2 s and goes away
   by itself; nothing reloads. (A hub restart starts a new session; a network drop resumes.)
7. **Server settings** (server menu → Server settings): rename the server, change or remove the
   icon; the Audit log lists every change with who did it; Invites lists active links and revokes
   them; Members offers the same actions as the member list.
8. **Moderation.** Kick the second user: their rail loses the server at once with a notice. Invite
   them again, then ban them with a reason: they lose the server, the invite page tells them they
   are banned, and Bans lists them until you revoke the ban.
9. **Owner rules.** Transfer ownership to another member (you become admin); the Danger zone
   deletes a server only after typing its name. An admin cannot kick another admin (the action is
   missing; the hub answers `cannot_target_admin`).
10. **Profile and appearance.** Settings → Profile → Change avatar: crop and apply; the avatar shows
    in the user panel and, for others, in the member list. Settings → Appearance: switch to Light
    and Compact, reload: no dark flash, the choice stays.
11. **Language.** With the browser in Portuguese (Brazil), every new screen, dialog, menu, notice
    and error is in pt-BR; default channel names of a new server are Portuguese.

### Phase 1

Prerequisites: `task setup` (now also installs sqlc and goose).

1. `task check` is green, and `task e2e` passes 7 Playwright tests.
2. **First run.** Delete `.dev/data` and run `task dev`. The hub log prints
   `setup required … /setup?token=dev-setup-token`. Open <http://localhost:5173>: it redirects to
   "This instance isn't set up yet" and offers a token field (you are on the host). Enter
   `dev-setup-token`.
3. **Wizard.** Give an instance name and keep the public URL. Try the admin password
   `password123`, go to the end and press **Finish setup**: the wizard jumps back to the admin step
   with "Too easy to guess". Use a stronger password and finish; you land on Home signed in, and the
   user panel shows your username. Open <http://localhost:5173/setup?token=dev-setup-token> again:
   it redirects to Home. `curl -i "http://127.0.0.1:7300/setup?token=x"` → `303`.
4. **Invite and registration.** Settings (gear) → Instance → **Create invite link**, then copy it.
   In a private window open `/register` without the invite: submitting shows "Required." under
   Invite code. Open the copied link: the code is pre-filled. Registration lands on "Check your
   inbox", because the dev mail log counts as working email. The confirmation link is in the hub
   log; opening it shows "Email confirmed", and **Continue** goes Home. Back in the admin window,
   the invite shows "Used …" and Users lists the new account.
5. **Login errors and limits.** In the private window, sign out and sign in with a wrong password,
   then with an unknown username: the message is identical. Repeat the wrong password 5 more
   times: "Too many attempts. Try again in N s."
6. **Email (dev mail log).** In Settings → Instance → Email, leave SMTP off; the dev hub logs
   emails. In a private window use "Forgot your password?" with the new account's email. The hub
   log shows the reset email with its link; open it, set a new password, and the old sessions of
   that account are signed out. **Real provider:** enable SMTP with your provider (Gmail:
   `smtp.gmail.com`, 587, STARTTLS, app password), **Save**, then **Send test email**; it should
   arrive, otherwise the server's reply is shown. Register a new account: it lands on "Check your
   inbox", the confirmation link works once, and a second click says the link is invalid.
7. **Without email.** Turn SMTP off again and run the hub without `--mail-dev-log`
   (`.dev/hub run --data-dir .dev/data`): the log warns that email is not configured, "Forgot your
   password?" says the instance can't send email, and the admin's Users list gives a working
   **Password reset link**.
8. **Account settings.** Change the display name and status (the user panel updates); change the
   password (other sessions end, this one stays); Sessions lists each browser, and **Sign out** on
   another session ends it there on its next request.
9. **Admin rules.** An admin cannot disable or demote themself (the buttons are absent; the RPC
   answers `cannot_target_self`). Disabling a user signs them out immediately.
10. **Deep links and language.** Signed out, open `/settings/sessions`: you get the login page;
    after signing in you are back on Sessions. With the browser in Portuguese (Brazil) every screen,
    error and email is in pt-BR.

### Phase 0

These steps describe Phase 0. Since Phase 1, Home needs a completed setup and a signed-in user.

Prerequisites: Go 1.27, Node 24, pnpm 12, Task 3.

1. Fresh setup: `task setup`. Expect buf, golangci-lint and livekit-server in `.tools/bin`; run it
   again and each tool reports "up to date".
2. `task check`. Expect "0 issues" from golangci-lint for `hub` and `tools`, clean ESLint and
   Prettier, all Go and Vitest tests passing, and "generated code is up to date".
3. Contract freshness: add a field to `CheckResponse` in `proto/dislackso/v1/health.proto`, run
   `task gen:check`, and expect it to fail and list the stale files. Revert, or run `task gen`.
4. Dev loop: `task dev`, open <http://localhost:5173>.
   - The shell shows the rail, the sidebar and "Welcome to Dislackso". The "Hub connection" card
     shows "Hub online", version `dev`, a latency in ms and a start time.
   - For the next check, stop `task dev` and run `task dev:web` and `task dev:hub` in two
     terminals. Press Ctrl+C in the hub terminal: within ~5 s the card and the sidebar badge switch
     to "Hub unreachable". Run `task dev:hub` again: it returns to "Hub online" without a reload.
   - Edit a string in `apps/web/src/core/i18n/locales/en.ts`; the page hot-reloads.
5. Language: set the browser's preferred language to Portuguese (Brazil) and reload; the UI is in
   pt-BR and `<html lang="pt-BR">`.
6. Single binary: `task build`, copy `bin/hub(.exe)` to an empty folder and run `hub run` there.
   - <http://127.0.0.1:7300> serves the same app (no Vite) with the hub online.
   - <http://127.0.0.1:7300/channels/1/2> loads the app and shows "Page not found" (client route
     fallback); "Back to home" works.
   - `curl -i http://127.0.0.1:7300/healthz` → `200 {"status":"ok"}`; `/readyz` →
     `{"status":"ready"}`; `/assets/nope.js` → 404.
   - `hub version` prints the git version; `hub run --help` lists the flags and their `DSX_*`
     variables.
   - Ctrl+C logs "shutting down" and exits cleanly.
7. Config precedence: create `config.yaml` in the data dir with `http: { addr: 127.0.0.1:7400 }`;
   the hub listens on 7400. `DSX_HTTP_ADDR=127.0.0.1:7500` overrides it, and
   `--http-addr 127.0.0.1:7600` overrides both. A typo such as `htp:` in the file stops startup with
   an error.
8. `task e2e`: the Playwright tests pass against the built binary.
9. CI: push a branch and open a PR. The lint, Go tests (ubuntu + windows), web and build jobs are
   green.
