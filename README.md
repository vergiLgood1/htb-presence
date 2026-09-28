# htb-presence

Discord Rich Presence for [Hack The Box](https://www.hackthebox.com/), written in Go.

`htb-presence` runs quietly in the background and shows your current HTB activity on your
Discord profile — the machine you're working on and how long you've been at it — without
you having to update anything by hand.

## Features

- 🟢 Live Discord Rich Presence for the active machine, season machine, or spawned challenge
- 🧩 Machine name, OS and difficulty on the details line, rank/points/flags on the
  state line, plus your HTB rank and points
- ⏱️ Elapsed-time playing timer; instance expiry moves to the avatar hover text
- 🔗 Buttons that open the machine or challenge and your HTB profile
- 🔒 Privacy toggles: hide the target name, rank, points, timer, or flag progress
- 🌙 Clear presence when nothing is spawned, or show a custom idle line
- 🛰️ "On the VPN" when a lab is not spawned but the HTB VPN is up
- 🔄 Automatic reconnect to Discord IPC and backoff on HTB API errors (honors `Retry-After`)
- ⚙️ YAML config, environment overrides, `init`, and a local session summary
- 🖥️ Single static binary, cross-platform (Linux, macOS, Windows)

## Status

Working v1: the core loop (poll HTB → map → push to Discord) runs end-to-end. See
[`docs/roadmap.md`](docs/roadmap.md) for the phased plan and what is still open.

Known limitations:

- Sherlock, Fortress, Endgame, and Pro Lab sessions are not detected as their own
  activity. A Fortress or Endgame VPN can still show up as "On the VPN" when
  `/connection/status` names that product. Starting Point boxes are machines and
  show up through the active-machine check.
- `/challenge/active` is not in HTB's published surface. A 404 there is treated as
  "no challenge" so machine presence keeps working.
- Discord Rich Presence requires the Discord **desktop** client running locally (it does
  not work with Discord in a browser).
- The logo and OS badges are Discord application art assets. Upload them or the
  images stay blank. See [Discord art assets](#discord-art-assets).

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

`htb-presence -init` writes that file (mode `600`) and prints the asset checklist.
`HTB_API_TOKEN` and `DISCORD_CLIENT_ID` override the file when they are set.

```yaml
htb:
  api_token: "your-htb-app-token"
  poll_interval: 30s        # minimum 15s
  vpn_fallback: true        # "On the VPN" when nothing is spawned

discord:
  client_id: "your-discord-application-id"
  show_machine_name: true   # also hides challenge names, avatars, and target links
  show_rank: true
  show_points: true
  show_timer: true          # elapsed playing time; expiry moves to avatar hover
  show_flags: false         # "user" / "root" owns; off because it is spoilery
  show_buttons: true
  clear_when_idle: false    # clear presence instead of the idle line
  idle_text: "Browsing…"

history:
  file: ""                  # optional JSONL session log; empty disables it
```

The App Token is never logged in full — it is redacted as e.g. `eyJ0…`.

## Usage

```bash
htb-presence -init                    # write a starter config and exit
htb-presence                          # run the background presence loop
htb-presence -once                    # fetch activity once and print it (smoke test)
htb-presence -stats                   # summarize history.file
htb-presence -version                 # print the version
htb-presence -config /path/config.yaml
```

Stop it with Ctrl-C (SIGINT) or SIGTERM; it clears the Discord presence on exit.

To keep it running after logout, install the user service in
[`docs/examples/htb-presence.service`](docs/examples/htb-presence.service) (Linux) or
[`docs/examples/htb-presence.plist`](docs/examples/htb-presence.plist) (macOS). Edit
`ExecStart` / `ProgramArguments` so they point at your binary.

## Discord art assets

In the [developer portal](https://discord.com/developers/applications), open your
application → Rich Presence → Art Assets and upload:

| Key | Used for |
|---|---|
| `htb` | Logo, shown as the large image when a target avatar is hidden or missing |
| `linux`, `windows`, `freebsd`, `openbsd`, `solaris` | Optional OS badge on the small image |

Asset names are case-sensitive and must match those keys. Without `htb`, presence
still works; the image is just blank.

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
