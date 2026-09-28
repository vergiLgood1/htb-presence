# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/) and this project adheres to
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed

- Target OS and difficulty moved from the state line to the details line, next to
  the target name: `Layover (Linux · Medium)`. The state line now only carries the
  user standing (rank/points/flags), so it stays short instead of crowding one
  line with system, standing, and expiry data.
- The presence timer is now always the elapsed playing time. The instance expiry
  no longer renders as a countdown; it rides along in the avatar hover text as
  `Layover · ends 3 Oct 17:50`.

## [0.2.0] - 2026-09-28

### Added

- Active challenge detection via `/challenge/active` (a 404 is treated as "no
  challenge") and season machines via `/season/machine/active`.
- The active machine or challenge avatar is shown as the presence large image, with
  the uploaded `htb` logo as the fallback. Hidden together with the target name when
  `show_machine_name: false`.
- Presence countdown from the instance `expires_at`, Discord link buttons, and an
  OS badge (`linux`, `windows`, `freebsd`, `openbsd`, `solaris`) on the small image.
- Config: `show_points`, `show_flags` (off by default), `show_buttons`,
  `clear_when_idle`, `idle_text`, and `vpn_fallback`.
- "On the VPN" when nothing is spawned. HTB `/connection/status` is tried first;
  if that call fails, a local route to an HTB lab prefix is used instead.
- `htb-presence -init`, `HTB_API_TOKEN` / `DISCORD_CLIENT_ID` overrides, and
  `htb-presence -stats` for the JSONL session log.
- `HTB_VPN_LIVE=1 go test ./internal/vpn -run Live -v`, which reads the real route
  table and logs every route it saw instead of only exercising the parsers.
- Example user service files in `docs/examples/`.
- Acceptable Use Policy notes in `docs/legal.md` (reviewed 2026-09-28).

### Changed

- `show_machine_name: false` also hides challenge names, target avatars, and the
  target button. Rank and points are separate toggles; points stay on when
  `show_rank` is on, matching previous presence text.
- A machine profile is cached for 10 minutes (`DefaultProfileTTL`), so a poll spends
  one request instead of two for the length of a session. User/root flag markers can
  lag by up to that window.

### Fixed

- A connection-status body of `{"status":"0","connection":"not connected"}` was
  rejected as an unexpected response, because the `connection` field was decoded into
  a struct. The API's authoritative "not connected" answer is now honored instead of
  falling back to the local route check.
- A Discord handshake that is refused is no longer reported as "client is not running".
  Discord accepts the socket and hangs up when it does not know the `client_id`, and
  that message used to be overwritten by the last dead candidate path (`/tmp/discord-ipc-9`
  and friends), sending anyone debugging it to the wrong place. The error now names the
  socket that answered and points at `discord.client_id`, and a silent hang-up reports
  `discord: connection closed` instead of a bare `EOF`.

### Documentation

- The vendored community API collection in `docs/api/` records that
  `https://www.hackthebox.com/api/v4` no longer serves the API (every v4 path answers
  404) and that it has no endpoint for a spawned challenge instance.

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

- Sherlock, Fortress, Endgame, and Pro Lab are not tracked as their own activity.
- Windows and macOS builds are cross-compiled but not runtime-tested.
- The Discord large-image asset (`htb`) must be uploaded to the Discord application for
  the logo to render.

[Unreleased]: https://github.com/vergiLgood1/htb-presence/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/vergiLgood1/htb-presence/releases/tag/v0.2.0
[0.1.0]: https://github.com/vergiLgood1/htb-presence/releases/tag/v0.1.0
