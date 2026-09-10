# 03 — Parity Matrix

Status legend: `—` not started · `WIP` in progress · `OK` ported and tested ·
`N/A` not applicable to the port.

## Feature parity

| Feature | Upstream | Go | Platform | Notes |
| --- | --- | --- | --- | --- |
| Loopback router + path token auth | OK | — | All | Constant-time compare |
| Model-name routing (DeepSeek vs GPT) | OK | — | All | |
| GPT OAuth passthrough | OK | — | All | Body untouched |
| DeepSeek body transform | OK | — | All | Effort, strips, parallel off |
| Tool-call replay repair | OK | — | All | Pair on `call_id` |
| Reasoning cloning within a turn | OK | — | All | Never across turns |
| Compaction seal/open + trigger flow | OK | — | All | AES-256-GCM |
| GPT vision image rewrite | OK | — | All | OAuth-borrowed |
| Request limits + encodings | OK | — | All | gzip/deflate/br/zstd |
| SSE streaming + disconnect abort | OK | — | All | Flush per chunk |
| Authenticated shutdown | OK | — | All | `_dscodex/shutdown` |
| Catalog merge (`dscodex-models.json`) | OK | — | All | |
| `config.toml` managed block | OK | — | All | Line-oriented |
| Key storage (DPAPI / 0600) | OK | — | Win / POSIX | |
| Proxy resolution + redaction | OK | — | All | Native in Go (no re-exec) |
| `start` / `serve` / `stop` / `status` | OK | — | All | PID-state trust model |
| `doctor` six checks | OK | — | All | |
| Autostart (launchd/systemd/schtasks) | OK | — | Per OS | |
| Supervisor crash-restart | OK | — | All | Windows autostart path |
| App-server bridge | OK | — | macOS | Opt-in |
| Provider/effort memory state | OK | — | macOS | `model-selections.json` |
| Picker/state smoke scripts | OK | — | Per OS | |

## Behavioral contracts (must match exactly)

- [ ] Router URL: `http://127.0.0.1:<port>/<43-char-token>/v1`
- [ ] 404 for a missing/incorrect token; no information leak
- [ ] Effort clamp: `low|medium|high` → `high`, everything else → `max`
- [ ] `store=false`, `parallel_tool_calls=false` always forced
- [ ] Stripped keys: `previous_response_id`, `conversation`, `background`,
      `metadata`, `service_tier`, reasoning `summary`/`generate_summary`/`context`
- [ ] Tool output directly follows its call; other items keep relative order
- [ ] Reasoning clones only within one turn
- [ ] Compaction prefix + layout `iv(12)|tag(16)|ciphertext`; key = SHA-256(token)
- [ ] Undecryptable compaction items dropped, never forwarded
- [ ] Vision only with `authorization`; placeholder text on failure
- [ ] 64 MiB compressed / 128 MiB decoded caps → 413
- [ ] `Upgrade` requests → 426
- [ ] Keep-alive 120 s, headers timeout 125 s
- [ ] Key never printed; proxy credentials redacted
- [ ] `stop` only via authenticated endpoint; never signal unverified PIDs
- [ ] `config.toml` edits never reformat user content

## Intentional differences

| Difference | Impact | Rationale |
| --- | --- | --- |
| No `--use-env-proxy` re-exec | None observable | Go supports env proxies natively |
| JSON object key order differs | None; consumers parse JSON | Go `encoding/json` sorts keys |
| Node version gate not applicable | None | No Node runtime |
| Autostart embeds binary path, not node+script | Reinstall if binary moves | Static binary |
| `serve` does not set `DSCODEX_PROXY_REEXEC` | None | Node-only mechanism |
| Picker display names are plain (`DeepSeek V4 Flash`) | Cosmetic; slugs, wire models, and routing unchanged | Product choice for DSCodex-Go |

## Test parity suites

| Upstream test | Go test | Status |
| --- | --- | --- |
| `app-server-state.test.mjs` | `internal/bridge/state_test.go` | — |
| `autostart.test.mjs` | `internal/autostart/autostart_test.go` | — |
| `config.test.mjs` | `internal/codexconfig/config_test.go` | — |
| `keys.test.mjs` | `internal/keystore/keys_test.go` | — |
| `platform.test.mjs` | `internal/constants/platform_test.go` | — |
| `proxy-config.test.mjs` | `internal/proxycfg/proxycfg_test.go` | — |
| `proxy.test.mjs` | `internal/router/proxy_test.go` | — |
| `real-codex.test.mjs` | `internal/bridge/realcodex_test.go` | — |
| `supervisor.test.mjs` | `internal/supervisor/supervisor_test.go` | — |
| `vision.test.mjs` | `internal/vision/vision_test.go` | — |
| `scripts/picker-smoke.mjs` | `scripts/smoke-picker` | — |
| `scripts/state-smoke.mjs` | `scripts/smoke-state` | — |

Differential fixtures generated from the upstream Node implementation live in
`testdata/` and are asserted on parsed structures, not bytes.
