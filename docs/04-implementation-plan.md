# 04 — Implementation Plan

Phases are tracked as epics in the GitHub Project (see `11-project-tracking.md`).
Each phase closes only when all sub-issues close and its gate passes.

**Estimate:** ~20 ideal days total.

## P0 — Scaffold & Tracking (1 day)

| Task | Detail |
| --- | --- |
| P0.1 | Go module, repository layout, Makefile, main stub |
| P0.2 | LICENSE, NOTICE, README, AGENTS.md |
| P0.3 | Documentation skeleton (00–04, 11) |
| P0.4 | CI matrix: gofmt, `go vet`, `go test -race` on 3 OSes |
| P0.5 | Backlog-as-code (`scripts/project/issues.json`) + bootstrap script |
| P0.6 | GitHub Project v2: fields, items, views/workflows checklist |
| P0.7 | Issue and PR templates |

**Gate:** `go build ./...` green locally, CI green, project populated.

## P1 — Foundation (2 days)

Port `constants`, `keystore` (config.json, DPAPI, tokens, legacy migration),
`codexconfig` (managed block, install/uninstall, bridge strip), `catalog`
(entry build/backfill, write/sync), and `proxycfg` (resolution, validation,
redaction, `NO_PROXY`).

Port the `keys`, `config`, `platform`, `proxy-config` suites and add catalog
golden tests.

**Gate:** unit suites green on Windows, macOS, and Linux; DPAPI round-trip
verified on Windows CI.

## P2 — Router Core (4 days)

HTTP/SSE server: token auth, byte caps, content-encoding decode, header
filtering, keep-alive/headers timeouts, `Upgrade` → 426, upstream abort on
disconnect, authenticated shutdown. DeepSeek body transform, compaction item
conversion, tool replay repair, reasoning cloning, and GPT passthrough.

Port `proxy.test.mjs` against `httptest` fakes; add streaming, cancel, and
limit tests.

**Gate:** proxy parity suite green; `go test -race ./...` clean.

## P3 — Vision & Compaction (2 days)

Vision describer (OAuth header borrow, cache, timeout, SSE parsing) wired into
the router; compaction seal/open, trigger flow, synthetic stream, and upstream
summary parsing.

Port `vision.test.mjs`; add seal/open round-trip and fake-upstream e2e tests.

**Gate:** vision and compaction tests green; no plaintext summary in streams.

## P4 — CLI & Lifecycle (2.5 days)

All commands: `install`, `sync`, `key`, `proxy`, `start`, `serve`, `supervise`,
`autostart`, `bridge`, `status`, `doctor`, `stop`, `uninstall`, `version`,
`help`. PID-state trust model, authenticated stop, hidden key prompt, doctor's
six checks.

**Gate:** manual `install → start → doctor` all `ok` on Windows; key never
printed; proxy redacted; stop leaves the router down.

## P5 — Autostart & Supervisor (2 days)

launchd plist, systemd unit, Windows VBS + Task Scheduler registration with
rollback semantics; supervisor crash-restart loop (exit 0 stays down).

Builder golden tests; Windows Task Scheduler smoke.

**Gate:** `autostart enable|disable|status` behaves correctly per OS.

## P6 — App-Server Bridge (2 days, macOS)

JSONL rewrite wrapper, provider/effort state machine with config
reconciliation and `staleEffort`, bridge shim and launchctl wiring,
stock-Codex resolution chain.

Port the state and picker smoke scripts.

**Gate:** both smokes pass on macOS; non-bridge platforms unaffected.

## P7 — Parity & Differential Validation (3 days)

Node-generated golden fixtures for catalog, config transforms, DeepSeek
bodies, compaction, and vision SSE. Remaining unit suites, race/stress pass,
real Codex CLI tool loops for both DeepSeek models, GPT passthrough
regression, macOS picker smoke, parity matrix completion.

**Gate:** 100% of the `03-parity-matrix.md` checklist verified.

## P8 — Release (1 day)

goreleaser config, version ldflags, tag-triggered release workflow, checksums,
install documentation, `v1.0.0` release candidate verification.

**Gate:** release candidate installs and passes `doctor` on all three OSes.

## Cross-phase rules

- One branch per phase (`phase-N-<slug>`); PRs use `Closes #N`.
- Definition of done per task: behavior matches upstream, tests added, docs
  updated, CI green, no secrets.
- Any divergence from upstream is recorded in `03-parity-matrix.md`.
- Upstream changes are tracked with the `upstream-sync` label (see
  `docs/10-upstream-sync.md` once created).
