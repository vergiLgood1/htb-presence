# Roadmap

This roadmap is organized in phases rather than dates, since this is an early-stage
side project. Check items off as they land; feel free to reorder within a phase.

## Phase 0 — Groundwork

- [x] Decide final repo layout: Go module at repo root, no `src/` (decided 2026-09-18,
      `AGENTS.md` updated).
- [ ] Generate a personal HTB App Token (Profile → Settings → App Tokens) and confirm
      which internal endpoint returns "current active machine/challenge", cross-checking
      `GoToolSharing/htb-cli` source and `Propolisa/htb-api-docs` (see
      `docs/architecture.md` §8) — HTB has no public API, so this must be verified
      hands-on rather than assumed from any doc.
- [ ] Register a Discord application to get a Rich Presence `client_id`.
- [x] Pick a license (MIT — see `LICENSE`).
- [ ] Note HTB's current ToS stance on this kind of automated polling.
- [x] `go.mod` + minimal `main.go` that builds and runs (no functionality yet).

## Phase 1 — Core loop (MVP)

- [ ] `internal/config`: load + validate config file, sane defaults.
- [ ] `internal/htb`: authenticate with the App Token and fetch current activity for the
      authenticated user against HTB's internal API (see `docs/architecture.md` §2.2).
- [ ] `internal/discord`: connect to local Discord IPC, send a static `SET_ACTIVITY`
      payload.
- [ ] `internal/presence`: poll loop wiring HTB → mapper → Discord, on a fixed interval.
- [ ] Manual end-to-end test: run locally, confirm presence shows up on your own
      Discord profile.

**Exit criteria:** running the binary with a valid config shows *something* (even a
static placeholder) as your Discord Rich Presence.

## Phase 2 — Real activity mapping

- [ ] Map actual HTB session data (active machine/challenge name) into the presence
      payload.
- [ ] Idle/fallback state when no active session.
- [ ] Change-detection so Discord is only updated when the activity actually changes
      (FR-7).
- [ ] Elapsed-time timer on the presence.
- [ ] Unit tests for the Presence Mapper (pure logic, no network/Discord needed).

**Exit criteria:** presence accurately reflects "what am I doing on HTB right now,"
updating promptly as it changes.

## Phase 3 — Resilience & polish

- [ ] Retry/backoff for HTB API failures (network, auth, rate limit).
- [ ] Retry/backoff + reconnect for Discord IPC drops.
- [ ] Clean shutdown on SIGINT/SIGTERM (clears presence).
- [ ] Structured logging with sensible levels; token masking.
- [ ] Privacy toggles: hide machine name / hide rank / etc., driven by config.

**Exit criteria:** the app can run unattended for days without manual intervention.

## Phase 4 — Distribution

- [ ] Cross-compile builds for Linux/macOS/Windows.
- [ ] GitHub Actions (or equivalent) release pipeline producing binaries.
- [ ] `go install`-able module path.
- [ ] README installation section filled in for real (currently marked "not yet
      published").

**Exit criteria:** someone other than the author can install and run this without
reading the source code.

## Phase 5 — Nice to have (post-v1, unscoped)

Ideas parked here are explicitly **out of scope** for v1 (see
`docs/requirements.md` §2.2) and only worth picking up once the above is stable:

- [ ] System tray icon / GUI wrapper instead of pure CLI.
- [ ] Optional local history of sessions (for personal stats, not shared).
- [ ] Package for common OS package managers (Homebrew, AUR, etc.).
- [ ] Config hot-reload without restart.

## Non-goals (won't do, at least not under this project)

- Posting activity to Discord channels/webhooks (this is Rich Presence, not a bot).
- Team dashboards or leaderboards.
- Multi-account support in a single running instance.
