# htb-presence

Discord Rich Presence for [Hack The Box](https://www.hackthebox.com/), written in Go.

`htb-presence` runs quietly in the background and shows your current HTB activity on your
Discord profile — the machine or challenge you're working on, your rank, and how long
you've been at it — without you having to update anything by hand.

## Features (planned)

- 🟢 Live Discord Rich Presence while an HTB machine/challenge session is active
- 📊 Rank, points, and season progress shown as presence details
- ⏱️ Session elapsed-time timer
- 🔄 Automatic reconnect to Discord IPC and graceful handling of HTB API rate limits
- ⚙️ Simple YAML/TOML config file (API key, refresh interval, privacy toggles)
- 🖥️ Single static binary, cross-platform (Linux, macOS, Windows)

## Status

🚧 Early planning stage — see [`docs/roadmap.md`](docs/roadmap.md) for where the project
currently stands, and [`docs/requirements.md`](docs/requirements.md) for the scope.

## Installation

> Not yet published. Once the first release is out, this section will cover:
> - `go install github.com/<you>/htb-presence@latest`
> - Downloading a prebuilt binary from Releases

## Configuration

`htb-presence` will read a config file from `$XDG_CONFIG_HOME/htb-presence/config.yaml`
(or `%APPDATA%\htb-presence\config.yaml` on Windows). Example (subject to change):

```yaml
htb:
  api_token: "your-htb-app-token"
  poll_interval: 30s

discord:
  client_id: "your-discord-application-id"
  show_machine_name: true
  show_rank: true
  show_timer: true
```

## How it works (short version)

1. `htb-presence` polls the HTB API for your current activity.
2. It maps that activity into a Discord Rich Presence payload.
3. It pushes the payload to the local Discord client over IPC.

See [`docs/architecture.md`](docs/architecture.md) for the full picture.

## Contributing

Contributions are welcome once the initial scaffold lands. If you (or an AI coding agent)
are working in this repo, please read [`AGENTS.md`](AGENTS.md) first — it documents the
conventions this codebase follows.

## Documentation

| File | Purpose |
|---|---|
| [`AGENTS.md`](AGENTS.md) | Ground rules and conventions for contributors (human or AI) |
| [`docs/requirements.md`](docs/requirements.md) | Functional & non-functional requirements |
| [`docs/architecture.md`](docs/architecture.md) | System design, components, data flow |
| [`docs/roadmap.md`](docs/roadmap.md) | Milestones and phased delivery plan |

## License

[MIT](LICENSE)
