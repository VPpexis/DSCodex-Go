# 02 — Port Mapping (Node → Go)

One Go package per upstream module. Every ported function keeps its upstream
name in Go style where practical, and its behavior is the contract.

## Module map

| Upstream | Go package | Notes |
| --- | --- | --- |
| `src/constants.mjs` | `internal/constants` | Models, host/port, `pathsFor`, `needsShellSpawn` |
| `src/keys.mjs` | `internal/keystore` | JSON store, DPAPI, tokens, legacy migration |
| `src/config.mjs` | `internal/codexconfig` | Managed block, install/uninstall, bridge strip |
| `src/catalog.mjs` | `internal/catalog` | Entry build/backfill, write/sync |
| `src/proxy-config.mjs` | `internal/proxycfg` | Resolution order, validate, redact, `NO_PROXY` |
| `src/proxy.mjs` | `internal/router` + `internal/compaction` | Server, transforms, SSE, shutdown |
| `src/vision.mjs` | `internal/vision` | Describe cache and rewrite |
| `src/app-server-state.mjs` | `internal/bridge` | Provider/effort state machine |
| `src/codex-wrapper.mjs` | `internal/bridge` | JSONL rewrite + spawn |
| `src/real-codex.mjs` | `internal/bridge` | Stock Codex resolution |
| `src/autostart.mjs` | `internal/autostart` | Builders + registration |
| `src/supervisor.mjs` | `internal/supervisor` | Restart loop |
| `src/cli.mjs` | `internal/cli` + `cmd/dscodex` | 15 commands, lifecycle, doctor |
| `scripts/*-smoke.mjs` | `scripts/` (Go) | Integration smokes |
| `test/*.test.mjs` | `*_test.go` | Parity suites |

## Function-level notes

### `constants.mjs`
- `DEEPSEEK_MODELS` picker slugs and wire models must match exactly — the
  catalog and routing depend on both slug forms. Display names intentionally
  differ (plain `DeepSeek V4 Flash` / `DeepSeek V4 Pro`; see
  `03-parity-matrix.md`).
- `pathsFor()` returns the same absolute paths; no new state files.
- `needsShellSpawn` only affects Windows `.cmd`/`.bat` execution.

### `keys.mjs`
- File shape: `deepseek_api_key`, `key_encoding` (`dpapi`/`plain`),
  `proxy_url`, `proxy_encoding`, `router_token`.
- Writes are atomic (temp file + rename) and 0600.
- `router_token` must match `^[A-Za-z0-9_-]{43}$`.
- Legacy plaintext Windows keys are migrated on the next state-changing call.
- DPAPI uses `CryptProtectData`/`CryptUnprotectData`, CurrentUser scope.

### `config.mjs`
- `managedRootBlock` recognizes the marker line followed by contiguous
  `openai_base_url` / `model_catalog_json` root keys.
- `stripManagedConfig` also removes the marker-owned
  `[desktop].enabled-reasoning-efforts` entry.
- `assertNoRootConflict` refuses to replace user-owned root keys.
- `injectDesktopReasoning` appends `[desktop]` when missing; if an existing
  `enabled-reasoning-efforts` lacks `max`, it errors instead of overwriting.
- Edits are line-oriented; the rest of the file is never reformatted.
- `stripBridgeCliPath` removes only DSCodex-owned `CODEX_CLI_PATH` values in
  `[mcp_servers.*.env]`.
- `ensureManagedRouterBinding` reconciles the managed URL with the persisted
  token and port at every runtime entry point.

### `catalog.mjs`
- The DeepSeek template is cloned from `gpt-5.6-sol` (fallback: first native
  entry); native entries get backfilled defaults (`prefer_websockets=false`,
  `supports_reasoning_summaries=false`, `base_instructions`).
- DeepSeek entries pin: `visibility=list`, `default_reasoning_level=max`,
  levels `high|max`, `input_modalities=["text","image"]`,
  `supports_parallel_tool_calls=false`, `context_window=1048576`,
  `apply_patch_tool_type="freeform"`, `web_search_tool_type="text"`, and the
  identity replacement in instructions.
- `additional_speed_tiers`, `service_tiers`, `default_service_tier` are deleted.
- Write is atomic JSON with 2-space indent.

### `proxy.mjs` → router
- Header allow-lists: OAuth set for ChatGPT, `user-agent` only for DeepSeek.
- Hop-by-hop response headers are dropped; `content-encoding`/`content-length`
  are re-derived.
- `authorizedPath` extracts the first path segment and constant-time compares.
- `buildDeepSeekBody`: effort clamp, `store=false`,
  `parallel_tool_calls=false`, deletes `previous_response_id`,
  `conversation`, `background`, `metadata`, `service_tier`, and reasoning
  `summary`/`generate_summary`/`context`.
- `normalizeToolCallReplay`: pair `function_call` / `custom_tool_call` /
  `local_shell_call` with their outputs on `call_id`, preserving the relative
  order of all other items; missing outputs stay put.
- `reasoningForExtraCalls`: a turn starts at a `reasoning` item, includes
  assistant messages, and ends at the first tool output; extra calls in the
  turn get a deep copy of the turn's reasoning.
- Compaction: `sealCompaction`/`openCompaction` (prefix
  `dscodex-compaction-v1:`), `buildDeepSeekCompactionBody`,
  `parseCompactionUpstream`, `sendCompactionStream`.
- Limits: 64 MiB compressed, 128 MiB decoded; 413 on exceed.
- Keep-alive 120 s, headers timeout 125 s; `Upgrade` → 426.

### `vision.mjs`
- Only OAuth identity headers are forwarded (`authorization`,
  `chatgpt-account-id`, `openai-beta`, `originator`, `session_id`,
  `user-agent`).
- Default model `gpt-5.6-sol`, override `DSCODEX_VISION_MODEL`.
- sha256-keyed LRU (200 entries); failures cached as empty strings.
- 60 s timeout; concurrent describes per request.
- No `authorization` header → body untouched.

### `app-server-state.mjs`
- State: `version=1`, `activeProvider`, per-provider effort slots, OpenAI
  `serviceTier`, `staleEffort`, `threads` (bounded, last 500).
- `reconcileWithConfig` adopts the live `config.toml` provider/effort/tier on
  load.
- `rewriteClient` handles `config/batchWrite`, `config/value/write`,
  `thread/start`, `thread/resume`, `thread/fork`,
  `thread/settings/update`; `rewriteServer` reconciles responses.
- Non-matching JSONL lines pass through unchanged.

### `codex-wrapper.mjs` / `real-codex.mjs`
- Resolution order: `launchctl getenv DSCODEX_REAL_CODEX`, process env,
  `/Applications/ChatGPT.app/Contents/Resources/codex`, then `PATH`.
- The wrapper never resolves to itself; `CODEX_CLI_PATH` is deleted from the
  child environment; non-app-server invocations are transparent pipes.

### `autostart.mjs`
- launchd: `com.dscodex.router` plist, `KeepAlive.SuccessfulExit=false`.
- systemd: `dscodex.service`, `Restart=on-failure`.
- Windows: UTF-16LE VBS shim + `Register-ScheduledTask` script, restart count
  255 / 1-minute interval.
- Generated artifacts never embed the DeepSeek key.

### `supervisor.mjs`
- Restarts `serve` on nonzero exit; exit 0 stays down; logs to `server.log`;
  clears `DSCODEX_PROXY_REEXEC` for children (Node-only; the Go port does not
  need the flag but keeps the environment clean).

### `cli.mjs`
Commands and semantics: `install`, `sync`, `key set|status|delete`,
`proxy set|status|clear`, `start`, `serve`, `supervise`, `autostart
enable|disable|status`, `bridge enable|disable|status`, `status`, `doctor`,
`stop`, `uninstall`, `version`, `help`. Exit codes and the six doctor checks
match upstream. `serve` owns the pid file regardless of who launched it.

## Porting rules

1. Read the upstream file before porting; keep behavior, not just shape.
2. Port upstream tests 1:1 where practical, then add Go-specific edge cases.
3. Any intentional divergence must be listed in `03-parity-matrix.md`.
