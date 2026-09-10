# DSCodex-Go

A Go port of [DSCodex](https://github.com/fish2lab/DSCodex): a local loopback
router that puts **DeepSeek V4 Flash / Pro** into the Codex and ChatGPT desktop
model picker while **GPT models keep working** through ChatGPT OAuth.

> **Status: P0 — scaffold.** The port is not usable yet. Track progress in the
> [project board](https://github.com/users/VPpexis/projects) and
> `docs/04-implementation-plan.md`.

## Why a Go port

- Single static binary — no Node.js runtime required.
- Native HTTP proxying, streaming, and proxy support.
- Drop-in compatible with existing DSCodex installs (`~/.codex` state, keys,
  router tokens, and catalog are reused unchanged).

## Planned behavior (parity target)

- DeepSeek V4 Flash / Pro in the Codex model picker via the native Responses API.
- Transparent GPT passthrough on `chatgpt.com` using the caller's OAuth headers.
- Full agentic tool loop: shell, `apply_patch`, function calls, web search.
- GPT-powered image description for the text-only DeepSeek models.
- Encrypted compaction handoff for DeepSeek-bound sessions.
- Optional autostart (launchd / systemd / Task Scheduler) and the macOS
  app-server bridge.

## Build and test

```sh
go build ./...
go vet ./...
go test -race ./...
```

Or via make: `make build`, `make test`, `make cross`.

## Documentation

| Doc | Contents |
| --- | --- |
| [docs/00-overview.md](docs/00-overview.md) | Goals, non-goals, glossary |
| [docs/01-architecture.md](docs/01-architecture.md) | Components and data flows |
| [docs/02-port-mapping.md](docs/02-port-mapping.md) | Node module → Go package map |
| [docs/03-parity-matrix.md](docs/03-parity-matrix.md) | Feature parity and contracts |
| [docs/04-implementation-plan.md](docs/04-implementation-plan.md) | Phased plan and gates |
| [docs/11-project-tracking.md](docs/11-project-tracking.md) | GitHub Project usage |

## License

MIT. See [LICENSE](LICENSE) and [NOTICE](NOTICE); DSCodex-Go is derived from
DSCodex, Copyright (c) 2026 fish2lab.
