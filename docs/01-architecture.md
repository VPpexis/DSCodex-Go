# 01 — Architecture

## Components

```
┌─────────────────────────────┐
│ Codex App / CLI / IDE       │
└──────────────┬──────────────┘
               │ POST /<token>/v1/responses  (HTTP/SSE, zstd, OAuth headers)
┌──────────────▼──────────────┐
│ internal/router             │  auth · body limits/decode · routing
│  ├─ deepseek transform      │  effort clamp, field strips, tool replay repair
│  ├─ internal/vision         │  GPT image describe (OAuth-borrowed)
│  ├─ internal/compaction     │  AES-256-GCM seal/open
│  └─ passthrough             │  GPT-bound traffic → chatgpt.com
└──────┬───────────────┬──────┘
       │               │
       ▼               ▼
 api.deepseek.com   chatgpt.com/backend-api/codex
```

| Package | Upstream module | Responsibility |
| --- | --- | --- |
| `internal/constants` | `constants.mjs` | Models, host/port, paths, platform helpers |
| `internal/keystore` | `keys.mjs` | `dscodex/config.json`: key, proxy, router token (DPAPI/0600) |
| `internal/codexconfig` | `config.mjs` | Line-oriented `config.toml` managed block |
| `internal/catalog` | `catalog.mjs` | Merge DeepSeek entries into `dscodex-models.json` |
| `internal/proxycfg` | `proxy-config.mjs` | Proxy resolution, validation, redaction, `NO_PROXY` |
| `internal/router` | `proxy.mjs` | HTTP/SSE router and body transforms |
| `internal/compaction` | `proxy.mjs` | Seal/open + synthetic compaction stream |
| `internal/vision` | `vision.mjs` | Image → text descriptions via GPT |
| `internal/bridge` | `app-server-state.mjs`, `codex-wrapper.mjs`, `real-codex.mjs` | JSONL rewrite + provider memory (macOS) |
| `internal/autostart` | `autostart.mjs` | launchd / systemd / Task Scheduler artifacts |
| `internal/supervisor` | `supervisor.mjs` | Crash-restart loop for Windows autostart |
| `internal/cli` | `cli.mjs` | Command implementations |
| `cmd/dscodex` | `cli.mjs` | Entry point only |

## Data flows

### DeepSeek request
1. Client POSTs to `http://127.0.0.1:10110/<router-token>/v1/responses`.
2. Router extracts and constant-time compares the path token; 404 otherwise.
3. Body is read with byte caps (64 MiB compressed / 128 MiB decoded) and
   decompressed (`gzip`, `deflate`, `br`, `zstd`).
4. If `model` matches a DeepSeek model:
   - `internal/router` clamps effort, strips unsupported fields, forces
     `store=false` / `parallel_tool_calls=false`, converts/drops compaction
     items, and repairs tool-call replay.
   - `internal/vision` replaces `input_image` parts with GPT descriptions when
     the request carries `authorization`; otherwise the body is untouched.
   - The request is forwarded to `api.deepseek.com/responses` with only
     `user-agent` plus the DeepSeek bearer key.
5. SSE streams back to the client with hop-by-hop headers stripped and each
   chunk flushed; a client disconnect aborts the upstream request.

### GPT passthrough
Non-DeepSeek models are forwarded to `chatgpt.com/backend-api/codex` with the
allow-listed OAuth headers intact and the body unmodified (including its
original `content-encoding`).

### Compaction
A DeepSeek-bound body containing `compaction_trigger` takes a dedicated path:
tools and the trigger are removed, a fixed summary prompt is appended, and the
same DeepSeek model is asked for a handoff summary. The summary is sealed as
`dscodex-compaction-v1:` + AES-256-GCM and returned as exactly one synthetic
`compaction` output item followed by `response.completed`. On later requests,
only DSCodex-prefixed items are decrypted (key = SHA-256 of the router token);
anything undecryptable is dropped, never forwarded raw.

### Lifecycle
`install` writes the marker-managed `config.toml` keys and the catalog.
`serve` reconciles the managed URL with the persisted token/port, writes
`server.pid` (pid, port, router token, shutdown token, instance id), and serves.
`stop` reads that state and calls the authenticated shutdown endpoint; it never
signals an unverified PID. `start` spawns a detached `serve`. Autostart wraps
`serve` in launchd / systemd / Task Scheduler.

## State and storage

| Path | Purpose | Mode |
| --- | --- | --- |
| `~/.codex/config.toml` | Codex config; DSCodex owns only marker-managed keys | user file |
| `~/.codex/dscodex-models.json` | Merged model catalog | 0600 |
| `~/.codex/dscodex/config.json` | DeepSeek key, proxy, router token | 0600 / DPAPI |
| `~/.codex/dscodex/model-selections.json` | Bridge provider/effort memory | 0600 |
| `~/.codex/dscodex/server.pid` | Router instance state | 0600 |
| `~/.codex/dscodex/server.log` | Router log | 0600 |
| `~/.codex/config.toml.pre-dscodex.bak` | One-time backup before first install | — |

## Trust boundaries

- **Loopback only.** The router binds `127.0.0.1`; the path token is the only
  authentication and must be compared in constant time.
- **Secrets.** The DeepSeek key and proxy credentials live only in
  `dscodex/config.json` (DPAPI on Windows, 0600 elsewhere). Generated
  autostart artifacts never embed the key.
- **PID trust.** `stop`/`uninstall` act only on a well-formed, authenticated
  pid-state file; a mismatched or unverifiable process is never signalled.
- **Compaction.** Sealed with a key derived from the router token; foreign or
  rotated ciphertext is dropped rather than forwarded.
