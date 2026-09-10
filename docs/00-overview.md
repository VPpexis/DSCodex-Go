# 00 — Overview

## What DSCodex is

DSCodex is a local, loopback-only router that makes **DeepSeek V4 Flash and
Pro** appear in the Codex / ChatGPT desktop model picker, the Codex CLI, and
the IDE extension, while **GPT models keep working** through ChatGPT OAuth.

```
Codex App / CLI / IDE
        │  HTTP/SSE (zstd, OAuth headers)
        ▼
http://127.0.0.1:10110/<router-token>/v1     ← DSCodex local router
        │
        ├── DeepSeek models → api.deepseek.com/responses
        │     (images described by GPT first, then injected as text)
        └── other models    → chatgpt.com/backend-api/codex (OAuth passthrough)
```

Upstream: <https://github.com/fish2lab/DSCodex> (Node.js, MIT, v1.1.0).

## Goal of this repository

A **single static Go binary** with full behavioral parity, targeting macOS,
Linux, and Windows, and **drop-in compatible state** with existing DSCodex
installs: same `~/.codex` paths, file formats, keys, and router tokens.

## Non-goals

- New features beyond upstream parity (a port, not a fork).
- A GUI; the CLI and file contracts are the interface.
- Supporting the ChatGPT web app (the router serves the local Codex runtime).
- Replacing the Codex client's own model list logic beyond the catalog merge.

## Locked decisions

| Decision | Choice | Rationale |
| --- | --- | --- |
| Repo / module | `DSCodex-Go` / `github.com/VPpexis/dscodex-go` | Conventional naming; module path lowercase |
| Binary | `dscodex` | Drop-in CLI parity with upstream |
| CLI | stdlib `flag` + manual dispatch | Upstream has zero runtime deps; keep the ethos |
| Platform scope | macOS, Linux, Windows | Upstream supports all three |
| Feature scope | Full parity incl. macOS bridge | Phase P6 |
| State | Drop-in compatible with Node installs | Users keep keys, tokens, and config |
| Baseline toolchain | Go 1.21 (`go.mod`) | Matches maintainer toolchain; bump in P2+ if deps require |
| Dependencies | zstd, brotli, x/sys, x/term only | Everything else is stdlib |
| Tracking | GitHub Projects v2 | See `11-project-tracking.md` |

## Intentional differences from upstream

1. **No `--use-env-proxy` re-exec.** Go's `http.Transport.Proxy` handles proxy
   environment variables natively; the resolution order and `NO_PROXY`
   semantics are still ported exactly (`internal/proxycfg`).
2. **No Node runtime.** Releases are cross-compiled static binaries.
3. **JSON key order may differ.** Upstream writes `JSON.stringify(..., 2)`;
   Go's `encoding/json` sorts object keys. Consumers parse JSON, and tests
   assert on parsed structures, never bytes.
4. **Binary path is baked into autostart artifacts.** Upstream embeds
   `node + cli.mjs`; the port embeds the `dscodex` binary path. `doctor`
   reports a broken binding.

## Glossary

| Term | Meaning |
| --- | --- |
| Router token | 43-char base64url secret in the loopback URL path; also the compaction key seed |
| Managed block | The `# DSCodex managed` marker lines in `config.toml` |
| Compaction item | `dscodex-compaction-v1:` sealed AES-256-GCM summary returned to Codex |
| Bridge | Optional macOS-only app-server wrapper that remembers provider/effort choices |
| Passthrough | GPT-bound traffic forwarded to `chatgpt.com` with OAuth headers intact |
| Catalog | `~/.codex/dscodex-models.json` merged into the Codex model picker |

## References

- Upstream source: <https://github.com/fish2lab/DSCodex>
- DeepSeek Responses API: <https://api-docs.deepseek.com/guides/responses_api/>
- Codex manual: <https://developers.openai.com/codex/codex-manual.md>
