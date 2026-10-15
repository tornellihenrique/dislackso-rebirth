# Status

## Current phase

**Phase 0 — Foundation: implemented, waiting for review.** Next: Phase 1 — Accounts & instance
setup.

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

## Known issues

- No `LICENSE` file yet: the license is the owner's choice.
- The web bundle is ~505 kB minified (156 kB gzip), so Vite prints its 500 kB chunk warning on
  build. Route-level code splitting will come with real routes.
- Go's `Server.Shutdown` waits up to 5 s for connections a client opened but never used (for
  example a browser preconnect), so stopping the hub can take a few seconds.
- `task dev` does not start LiveKit yet (Phase 3), and `task test:int` has no tests yet.
- Lessons from the reference repo still to apply: apply the persisted theme before the first render
  to avoid a flash (Phase 2 Appearance settings); keep the remote-audio elements mounted for as long
  as the user is in the room, not tied to the stage view (Phase 3).

## Manual test script

### Phase 0

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
8. `task e2e`: 3 Playwright tests pass against the built binary.
9. CI: push a branch and open a PR. The lint, Go tests (ubuntu + windows), web and build jobs are
   green.
