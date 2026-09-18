# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/) and this project adheres to
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - 2026-09-18

First public release.

### Added

- Poll HTB's internal API (App Token auth) for the active machine
  (`/machine/active`, `/machine/profile/{id}`).
- Discord Rich Presence over local IPC — Unix socket on Linux/macOS, named pipe on
  Windows — with handshake, activity updates and presence clearing on shutdown.
- Presence content: machine name, OS, difficulty, elapsed-time session timer, and HTB
  rank/points (cached for 10 minutes).
- Privacy toggles: `show_machine_name`, `show_rank`, `show_timer`.
- YAML configuration with validation and a 15s poll-interval floor (30s default).
- Adaptive backoff for HTB API failures (exponential, honoring `Retry-After` on HTTP 429,
  slower fixed retry on auth failures) and reconnect with backoff for Discord IPC drops.
- Config hot-reload without restarting, via a polling watcher.
- Optional local session history as JSONL (`history.file`), off by default.
- CLI flags: `-config`, `-once`, `-version`.
- Structured logging with App Token masking.
- CI (gofmt/vet/test plus a Windows cross-build) and a tagged release pipeline producing
  Linux, macOS and Windows binaries with checksums.

### Known limitations

- Only the active machine is tracked; HTB challenge sessions are not detected.
- Windows and macOS builds are cross-compiled but not runtime-tested.
- The Discord large-image asset (`htb`) must be uploaded to the Discord application for
  the logo to render.

[Unreleased]: https://github.com/vergiLgood1/htb-presence/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/vergiLgood1/htb-presence/releases/tag/v0.1.0
