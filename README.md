# htb-presence

Discord Rich Presence for [Hack The Box](https://www.hackthebox.com/), written in Go.

`htb-presence` runs quietly in the background and shows your current HTB activity on your
Discord profile — the machine you're working on and how long you've been at it — without
you having to update anything by hand.

## Features

- 🟢 Live Discord Rich Presence while an HTB machine session is active
- 🧩 Machine name, OS and difficulty, plus your HTB rank and points
- ⏱️ Session elapsed-time timer
- 🔒 Privacy toggles: hide the machine name, hide rank, hide the timer
- 🔄 Automatic reconnect to Discord IPC and backoff on HTB API errors (honors `Retry-After`)
- ⚙️ Simple YAML config file
- 🖥️ Single static binary, cross-platform (Linux, macOS, Windows)

## Status

Working v1: the core loop (poll HTB → map → push to Discord) runs end-to-end. See
[`docs/roadmap.md`](docs/roadmap.md) for the phased plan and what is still open.

Known limitations:

- Only the *active machine* is tracked; HTB challenge sessions are not detected.
- Discord Rich Presence requires the Discord **desktop** client running locally (it does
  not work with Discord in a browser).

## Requirements

- The Discord desktop client, running and logged in.
- An HTB account with a personal **App Token**: `app.hackthebox.com` → Profile →
  Settings → App Tokens.
- A Discord application client ID (create one at
  <https://discord.com/developers/applications>).

## Installation

Requires Go 1.21+. With a working Go toolchain:

```bash
go install github.com/vergiLgood1/htb-presence/cmd/htb-presence@latest
```

Or clone and build:

```bash
git clone https://github.com/vergiLgood1/htb-presence
cd htb-presence
go build ./cmd/htb-presence
```

Tagged releases (`v*`) are built for Linux, macOS and Windows by
[`.github/workflows/release.yml`](.github/workflows/release.yml) and attached to the
GitHub release with checksums.

## Configuration

`htb-presence` reads a config file from
`$XDG_CONFIG_HOME/htb-presence/config.yaml` (`~/.config/htb-presence/config.yaml` by
default; `%APPDATA%\htb-presence\config.yaml` on Windows). Use `-config` to point it
somewhere else.

```yaml
htb:
  api_token: "your-htb-app-token"
  poll_interval: 30s        # minimum 15s

discord:
  client_id: "your-discord-application-id"
  show_machine_name: true   # privacy toggles
  show_rank: true
  show_timer: true

history:
  file: ""                  # optional JSONL session log; empty disables it
```

The App Token is never logged in full — it is redacted as e.g. `eyJ0…`.

## Usage

```bash
htb-presence                          # run the background presence loop
htb-presence -once                    # fetch activity once and print it (smoke test)
htb-presence -version                 # print the version
htb-presence -config /path/config.yaml
```

Stop it with Ctrl-C (SIGINT) or SIGTERM; it clears the Discord presence on exit.

## How it works (short version)

1. `htb-presence` polls HTB's internal API for your current activity.
2. It maps that activity into a Discord Rich Presence payload.
3. It pushes the payload to the local Discord client over IPC.

See [`docs/architecture.md`](docs/architecture.md) for the full picture.

## How it talks to HTB

HTB does not offer a public API for regular accounts. This project uses the same internal
(v4) endpoints HTB's own web app uses, authenticated with a user-generated App Token —
the same approach as community tools such as
[`GoToolSharing/htb-cli`](https://github.com/GoToolSharing/htb-cli). That API is
unofficial and may change without notice; please respect HTB's Terms of Service.

> **Disclaimer:** `htb-presence` is unofficial and is **not affiliated with or endorsed
> by Hack The Box**. It relies on HTB's internal API, which may break without notice. You
> are responsible for reviewing and complying with HTB's Terms of Service before using
> it — use at your own risk. See [`docs/legal.md`](docs/legal.md).

## Contributing

If you (or an AI coding agent) are working in this repo, please read
[`AGENTS.md`](AGENTS.md) first — it documents the conventions this codebase follows.

## Documentation

| File | Purpose |
|---|---|
| [`AGENTS.md`](AGENTS.md) | Ground rules and conventions for contributors (human or AI) |
| [`docs/requirements.md`](docs/requirements.md) | Functional & non-functional requirements |
| [`docs/architecture.md`](docs/architecture.md) | System design, components, data flow |
| [`docs/roadmap.md`](docs/roadmap.md) | Milestones and phased delivery plan |
| [`docs/legal.md`](docs/legal.md) | Unofficial-use disclaimer and ToS review checklist |
| [`CHANGELOG.md`](CHANGELOG.md) | Release history |

## License

[MIT](LICENSE)
