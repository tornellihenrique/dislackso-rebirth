# Status

## Current phase

**Phase 1 — Accounts & instance setup: implemented, waiting for review.** Next: Phase 2 — Guilds,
channels, realtime shell, basic moderation.

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
  to Phase 5's host experience.
- **Settings are a full-screen route** (`/settings/...`) rather than a modal over the app, so deep
  links and the back button work. Phase 2 can render it as a modal over the shell.

## Known issues

- **Client IP is the TCP peer.** That is correct for direct connections. A tunnel or reverse proxy
  on the same machine would make every client look local, though, which collapses per-IP rate
  limits and makes setup "local" (the setup token still protects it). Each exposure provider must
  supply its own `clientip.Resolver` in Phase 5.
- **The session cookie is `Secure` only when the request arrived over TLS.** Phase 5 must set it
  from the exposure provider, since tunnels terminate TLS before the hub.
- Expired sessions and used or expired email tokens are never purged yet; a periodic cleanup job
  is still to come.
- The status preference is stored but has no effect until presence exists (Phase 2).
- No `LICENSE` file yet: the license is the owner's choice.
- The web bundle is ~580 kB minified (178 kB gzip), so Vite prints its 500 kB chunk warning on
  build. Route-level code splitting will come with the larger features.
- Go's `Server.Shutdown` waits up to 5 s for connections a client opened but never used (for
  example a browser preconnect), so stopping the hub can take a few seconds.
- `task dev` does not start LiveKit yet (Phase 3), and `task test:int` has no tests yet.
- Lessons from the reference repo still to apply: apply the persisted theme before the first render
  to avoid a flash (Phase 2 Appearance settings); keep the remote-audio elements mounted for as long
  as the user is in the room, not tied to the stage view (Phase 3).

## Manual test script

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
