# AGENTS.md

This file gives coding agents (and human contributors) the context they need to work
effectively in this repository. If you are an AI agent picking up a task here, read this
file fully before making changes.

## Project summary

`htb-presence` is a Go CLI/background service that reads a user's Hack The Box (HTB)
activity and publishes it as Discord Rich Presence via Discord's local IPC protocol. It
is a single-purpose, long-running client tool — not a web service.

**Important:** HTB has no public API for regular accounts. This project talks to HTB's
*internal, unofficial* v4/v5 API using a user-generated App Token, the same way
`GoToolSharing/htb-cli` (Go) and `Pirrandi/htb-presence` (Python, same project name)
do. See `docs/architecture.md` §2.2 and §8 before touching anything in `internal/htb`.

Read `docs/requirements.md` for scope and `docs/architecture.md` for how the pieces fit
together before implementing anything non-trivial.

## Ground rules

1. **Keep it a single static binary.** No CGo unless absolutely required. Avoid runtime
   dependencies that aren't vendored/go-modules based.
2. **No secrets in the repo.** HTB API tokens and Discord client IDs are user config,
   never hardcoded, never logged in full (mask tokens in logs/errors).
3. **Fail soft, not loud.** This is a background presence tool — a failed HTB API call or
   a dropped Discord IPC connection should be retried with backoff, not crash the process.
4. **Don't over-poll.** Respect HTB API rate limits. Polling interval is configurable but
   should default to something conservative (≥ 15s).
5. **Small, focused packages.** Prefer `internal/htb`, `internal/discord`,
   `internal/config`, `internal/presence` over one large `main` package. See
   `docs/architecture.md` for the intended layout.

## Repository layout

```
htb-presence/
├── README.md          # user-facing overview
├── AGENTS.md           # this file
├── LICENSE
├── docs/
│   ├── architecture.md
│   ├── requirements.md
│   └── roadmap.md
├── cmd/
│   └── htb-presence/
│       └── main.go     # wiring: config → clients → scheduler
├── internal/           # created as packages land (config, htb, discord, presence)
└── go.mod
```

The Go module lives at the repo root (decided 2026-09-18); there is no `src/` directory.

## Build, run, test

These commands run from the repo root, where the Go module lives.

```bash
# Build
go build ./...

# Run
go run ./cmd/htb-presence

# Test
go test ./...

# Vet / lint
go vet ./...
gofmt -l .
```

Agents should run `go build ./...`, `go vet ./...`, and `go test ./...` before
considering a change complete. If `golangci-lint` is configured later, run that too.

## Code style

- Standard `gofmt`/`goimports` formatting — no exceptions.
- Exported identifiers get doc comments (`// Foo does X.`).
- Errors are wrapped with context (`fmt.Errorf("polling htb: %w", err)`), never silently
  swallowed.
- Prefer explicit dependency injection (pass clients/interfaces in) over globals, to keep
  the HTB client, Discord client, and config testable in isolation.
- Table-driven tests (`t.Run` subtests) for anything with multiple cases.

## Things an agent should NOT do without asking

- Adding a new third-party dependency for something the stdlib can do.
- Changing the config file schema without updating `README.md` and
  `docs/architecture.md` in the same change.
- Committing anything resembling a real API token, even in examples/tests.
- Renaming/moving top-level docs or the Go module layout without asking.

## Where to look first for a given task

| Task | Start here |
|---|---|
| HTB API polling / auth | `docs/architecture.md` → "HTB Client", then `internal/htb` (once it exists) |
| Discord Rich Presence payloads | `docs/architecture.md` → "Discord Client", `internal/discord` |
| Config file parsing | `docs/architecture.md` → "Config", `internal/config` |
| New feature scoping | `docs/requirements.md` |
| "What's next" / priorities | `docs/roadmap.md` |

## Open questions worth flagging to a human

- Exact endpoint(s) and response shape for "current active machine/challenge" — confirm
  against `GoToolSharing/htb-cli` source and `Propolisa/htb-api-docs` before
  implementing `internal/htb`, since HTB's API is unofficial/unversioned-for-us and may
  differ from what's written in `docs/architecture.md`.
- Whether to include the optional local VPN-detection fallback (see
  `docs/architecture.md` §2.2) in v1 or defer it — it adds OS-specific code
  (checking for a `tun`/OpenVPN interface) for a secondary signal.
- Whether HTB's current Terms of Service need a second look before shipping a public
  release, given the API is unofficial.
