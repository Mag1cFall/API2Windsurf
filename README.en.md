# API2Windsurf

English · [中文](README.md)

[![CI](https://github.com/Mag1cFall/API2Windsurf/actions/workflows/ci.yml/badge.svg)](https://github.com/Mag1cFall/API2Windsurf/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Mag1cFall/API2Windsurf?include_prereleases&sort=semver)](https://github.com/Mag1cFall/API2Windsurf/releases)
[![Downloads](https://img.shields.io/github/downloads/Mag1cFall/API2Windsurf/total)](https://github.com/Mag1cFall/API2Windsurf/releases)
[![License](https://img.shields.io/github/license/Mag1cFall/API2Windsurf)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/Mag1cFall/API2Windsurf)](go.mod)
[![Platform](https://img.shields.io/badge/platform-Windows%20x64-0078d4)](https://github.com/Mag1cFall/API2Windsurf/releases)

Bring your own model to Windsurf. Plug in any OpenAI-compatible, Anthropic, or Google Gemini endpoint, with thinking / reasoning passthrough.

A local relay: it intercepts the chat requests Windsurf sends to its cloud, rewrites them in a standard format, and forwards them to the API endpoint you configure. The Windsurf UI keeps working as is; what answers behind the scenes is the model you set. No Windsurf settings need to change.

## What it does

The program runs a local service that receives chat requests from Windsurf and translates them into upstream API calls over one of three wire protocols: OpenAI-compatible, Anthropic, or Google Gemini.

For models that emit reasoning (Claude thinking, DeepSeek R1, etc.), the reasoning content is written into Windsurf's native thinking field, so the IDE renders a collapsible thought block just like it does for first-party thinking models.

## Usage

1. Run `api2windsurf.exe` and accept the UAC prompt.
2. In the configuration window, fill in upstream API URL and key, pick the protocol and model.
3. Click "Test connection", then "Save and apply".
4. Fully quit Windsurf and reopen it.
5. Pick any model in Windsurf and start chatting; the request will be rewritten to the model you configured.

> Whatever model you pick inside Windsurf does not matter — the relay rewrites it to your configured one. The real "model switch" lives in this app.

**Switch back to the official account**: quit this app, hit "Stop proxy", or "Restore official environment". On exit, hosts entries are removed automatically so Windsurf can reach the official backend again. The local CA certificate is intentionally kept (harmless without the proxy running, avoids re-prompting UAC on next launch); remove it manually via `certmgr.msc` if you want a clean uninstall. Then **fully quit** and relaunch Windsurf processes.

See [docs/usage.md](docs/usage.md) for details.

## Build from source

Requires Go 1.25+, Node.js 18+, and [Wails v2](https://wails.io).

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails build
```

The output is `build/bin/api2windsurf.exe`, a single self-contained file.

Dev mode (frontend hot reload):

```bash
wails dev
```

## How it works

```
Windsurf IDE  ── chat request ──>  server.self-serve.windsurf.com
                                    │ (hosts -> 127.0.0.1)
                                    ▼
                             api2windsurf.exe
                             intercept -> decode -> translate -> forward
                                    │
                                    ▼
                              your API endpoint
```

The program edits the system hosts file to point Windsurf's backend domains at localhost, then terminates TLS on port 443 with a self-signed certificate. Only chat-related requests are rewritten and forwarded; login, account state, telemetry, and other traffic pass through unchanged so the rest of the IDE stays unaffected.

Protocol notes: [docs/cascade-protocol.md](docs/cascade-protocol.md) and [docs/architecture.md](docs/architecture.md).

## Project layout

```
api2windsurf/
├─ main.go              Wails entrypoint
├─ internal/app/        application logic (config, lifecycle, bound API)
├─ internal/proxy/      proxy core (certs, hosts, MITM server, provider translation)
├─ internal/protocol/   Cascade protobuf codec
├─ frontend/            config UI (React + Vite)
├─ docs/                protocol notes
└─ .github/workflows/   CI and release
```

## Tests

```bash
go test ./...
```

## License

MIT. See [LICENSE](LICENSE). Release history in [CHANGELOG.md](CHANGELOG.md).
