# CLAUDE.md

Self-hosted Discord-like app for a small dev team: voice channels, high-quality screen sharing with Slack-style drawing, later text chat. Any member hosts the backend ("hub") on their own PC with no port forwarding and no paid services.

Read before working:
1. `docs/ARCHITECTURE.md` — the spec. §2 is the locked decision ledger.
2. `docs/ROADMAP.md` — phases and acceptance criteria.
3. `docs/STATUS.md` — what is done, what is next, known issues.

## Workflow
- Work one roadmap phase at a time. Finish every acceptance box, run `task check`, update `docs/STATUS.md` (done items, decisions, known issues, manual test script), then stop and summarize.
- Never change a locked decision (ARCHITECTURE §2) on your own. If one blocks you, stop and explain the trade-off. Approved changes get an ADR in `docs/adr/` and an update to the ledger.
- Before using a library API you are not sure about (LiveKit, tsnet, Connect, Electron), check its current docs/source instead of guessing. Pin versions.
- Contract first: edit `proto/`, run `task gen`, commit generated code with the change.
- Commit at logical checkpoints with Conventional Commits (`feat(voice): ...`).
- When a phase or requested change is done and `task check` is green, push a branch and open a PR against `master`, then stop. The owner watches CI and merges: do not merge, enable auto-merge or keep checking CI. If CI fails, the owner will ask for a fix.

## Commands
```
task tools      # install pinned dev tools (buf, sqlc, goose, livekit-server → .tools/)
task dev        # hub (with dev LiveKit) + Vite dev server with proxy
task gen        # protobuf (Go + TS) and sqlc
task db:migration -- <name>   # new goose SQL migration in hub/internal/adapter/sqlite/migrations
task check      # lint + typecheck + unit tests (Go + TS) + gen freshness
task test:int   # Go integration tests with real livekit-server
task e2e        # Playwright with fake media
task e2e:desktop  # the desktop app against a fresh hub (Windows, real screen)
task desktop    # build and open the desktop app
task bench:share  # screen share codec and layer measurements (~10 min, not in CI)
task build      # web build embedded into hub binary at bin/ (livekit-server copied next to it); desktop build
```

## Repo map
```
proto/                  contracts (Buf) — single source of truth for every wire format
hub/                    Go backend: cmd/hub, internal/{app,config,domain,ports,service,adapter,transport,events,sidecar}, gen/
apps/web/               React client (core/* is framework-agnostic; features/*; stores/; ui/)
apps/desktop/           Electron shell (loads the instance URL; native bridge only)
packages/contracts/     generated TS
packages/annotation/    drawing engine + renderer shared by web and desktop overlay
e2e/                    Playwright
docs/                   ARCHITECTURE, ROADMAP, STATUS, HOSTING, adr/
```

## Code rules (all languages)
- SOLID. Small interfaces owned by the consumer. Business logic in services; transport layers and React components stay thin.
- Every external dependency (DB, mail, media server, TURN, tunnel, storage, clock, IDs) sits behind a port; concrete types appear only in the composition root (`hub/internal/app`, `apps/web/src/app`).
- Design new features with extension points for future variants (new providers, new channel types, roles) without rewriting callers.
- Match the existing style and naming of the file and package you are editing.
- Comments: write them as the project's developer. Only when the code cannot say *why* on its own (non-obvious constraint, browser/OS quirk, protocol detail). No comments that restate the code, narrate changes, reference prompts/tasks/phases/the assistant, or address the reader. Doc comments on exported symbols only when the name is not enough, one line preferred. No commented-out code.
- Domain term in code is `Guild`; UI text says "Server".
- No hard-coded user-facing strings: use i18n keys (`en` and `pt-BR`).

### Go
- Go latest stable, `gofumpt`, `golangci-lint` clean. Errors wrapped with context (`fmt.Errorf("load guild %d: %w", id, err)`); domain errors mapped to Connect codes only in `transport/rpc`.
- `context.Context` first parameter for anything doing I/O. No global state; dependencies injected via constructors.
- Tests: table-driven, stdlib `testing` + `go-cmp`. Services tested with fakes; repositories with real SQLite.

### TypeScript
- `strict`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `verbatimModuleSyntax`. No `any`; no non-null assertions except in tests.
- Named exports only. Components: PascalCase files, props `interface` above the component. Hooks `useX`. Pure modules kebab-case.
- `core/*` never imports React. Stores are updated only through dispatcher/reducer functions.
- Tailwind with design tokens (CSS variables); no raw colors/radii in components.
- Tests: Vitest for pure modules and reducers; Testing Library for critical components.

## Invariants (breaking these causes real bugs)
- The hub is the authority for voice state; LiveKit webhooks reconcile it. One voice connection per user (identity = userId).
- Media never goes through the HTTP exposure path; only signaling/API/files do.
- Gateway: only `GatewayDispatcher` writes entity stores from push events; reconnection always tries `Resume` first.
- The media engine (`core/media`) and drawing engine live outside React; React subscribes to stores.
- Drawing coordinates are normalized to the share's video content rect, never to the DOM element.
- Stroke author is the LiveKit packet sender, never a payload field.
- `<video>` elements receive tracks through callback refs; fullscreen tiles render through a portal.
- PTT releases on window blur. Speaking indicator uses a ~400 ms hangover.
- Deafen implies mute and restores the previous mute state on undeafen; both work outside calls.
- Secrets and session tokens never leave the hub in logs or API responses; public user shape is built in one mapper.
- Setup wizard is reachable only from localhost and only before setup completes.
- `livekit.yaml` is generated by the hub; never edited by hand.
