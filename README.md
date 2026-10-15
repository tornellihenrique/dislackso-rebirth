# Dislackso

Self-hosted, Discord-like app for a small dev team: voice channels, high-quality screen sharing with
live Slack-style drawing, and (later) text chat. Any member hosts the backend (the **hub**) on their
own PC: no static IP, no port forwarding, no paid services.

- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md): specification and locked decision ledger
- [`docs/ROADMAP.md`](docs/ROADMAP.md): phases and acceptance criteria
- [`docs/STATUS.md`](docs/STATUS.md): what is done, what is next, known issues

## Development

Prerequisites: Go 1.27, Node 24 LTS, pnpm 12 and [Task](https://taskfile.dev) 3. Everything else
(buf, golangci-lint, sqlc, goose, livekit-server) is pinned in `tools/manifest.json` and installed into `.tools/`
by `task tools`. Works the same on Windows, macOS and Linux.

```sh
task setup   # pinned tools + pnpm install
task dev     # hub + Vite dev server, open http://localhost:5173
task check   # lint, typecheck, unit tests, generated-code freshness
task build   # single hub binary with the web client embedded, at bin/hub
task e2e     # Playwright against the built binary
```

On a fresh `.dev/` data dir, `task dev` starts with setup pending: open
<http://localhost:5173/setup?token=dev-setup-token>. The dev hub writes emails (verification,
password reset) to its log instead of sending them.

`task --list` shows every task. The repository layout and coding rules are in
[`CLAUDE.md`](CLAUDE.md).

### Running the hub

```sh
bin/hub run --help
```

Settings come from defaults, then `config.yaml` in the data directory, then `DSX_*` environment
variables, then flags. By default the hub listens on `127.0.0.1:7300` and keeps its data in
`%APPDATA%\Dislackso\hub` (Windows), `~/Library/Application Support/Dislackso/hub` (macOS) or
`~/.local/share/dislackso/hub` (Linux).

On first start the hub prints a one-time setup link (`http://localhost:7300/setup?token=…`). The
wizard only opens on the host machine itself, and closes for good once the admin account exists.
Email is optional: without SMTP, verification is off and admins hand out password reset links.
