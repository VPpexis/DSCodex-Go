# AGENTS.md — working rules for DSCodex-Go

This repository is a Go port of [fish2lab/DSCodex](https://github.com/fish2lab/DSCodex)
(Node.js, MIT). The upstream repository is the behavioral specification. When in
doubt, read the upstream source file named in `docs/02-port-mapping.md` and
match its behavior exactly.

## Commands

| Task | Command |
| --- | --- |
| Build | `go build ./...` |
| Test | `go test -race ./...` |
| Vet | `go vet ./...` |
| Format | `gofmt -w .` |
| Cross-compile | `make cross` |
| Project bootstrap | `pwsh -File scripts/project/bootstrap.ps1` (or Windows PowerShell) |

Baseline toolchain: **Go 1.21** (the `go` directive in `go.mod`). Dependencies
added in P2+ may force a bump; if so, update `go.mod`, CI, and this file in the
same PR.

## Repository layout

- `cmd/dscodex/` — CLI entry point only; no business logic.
- `internal/<package>/` — one package per upstream module (see
  `docs/02-port-mapping.md`).
- `docs/` — all documentation; every behavior contract lives here.
- `scripts/project/` — backlog-as-code and GitHub Project bootstrap.
- `testdata/` — golden fixtures generated from the upstream implementation.

## Non-negotiable behavior contracts

1. **Secrets never leak.** Never print, log, or commit the DeepSeek API key or
   proxy credentials. Redact proxy URLs in CLI output. The stored key file is
   `~/.codex/dscodex/config.json` (mode 0600; DPAPI-encrypted on Windows).
2. **config.toml is user-owned.** Only the marker-managed root keys
   (`openai_base_url`, `model_catalog_json`), the marker-owned
   `[desktop].enabled-reasoning-efforts` entry, and DSCodex-owned
   `CODEX_CLI_PATH` values in `[mcp_servers.*.env]` may be rewritten. Never
   reformat the rest of the file; edits are line-oriented, never full-TOML
   round-trips.
3. **Router auth.** Every request must carry the 43-char base64url router token
   in the path; compare with a constant-time comparison. `stop` may only use the
   authenticated shutdown endpoint and must never signal an unverified PID.
4. **DeepSeek body rules.** Clamp effort (`low|medium|high` → `high`, else
   `max`), force `store=false` and `parallel_tool_calls=false`, strip
   `previous_response_id` / `conversation` / `background` / `metadata` /
   `service_tier`, re-pair tool calls with their outputs on `call_id`, and clone
   a turn's reasoning for extra calls only within that turn.
5. **Compaction.** Sealed as `dscodex-compaction-v1:` + AES-256-GCM
   (`iv(12)|tag(16)|ciphertext`, key = SHA-256(router token)). Items that fail
   to decrypt are dropped, never forwarded to DeepSeek.
6. **Vision.** Images are rewritten into GPT-generated text only when the
   incoming request carries ChatGPT OAuth headers; otherwise the body passes
   through untouched.
7. **Platform differences.** The app-server bridge is macOS-only and opt-in.
   Windows key storage uses DPAPI; POSIX uses 0600. Autostart uses
   launchd / systemd / Task Scheduler.

## Testing rules

- Port the upstream test suite 1:1 where practical (same cases, same names).
  Add table-driven tests in the same package as the code under test.
- Use `t.TempDir()` for filesystem tests; use `net/http/httptest` for upstream
  fakes. Never touch the real `~/.codex` in tests.
- Differential tests compare against golden fixtures generated from the
  upstream Node implementation (`scripts/`), asserting on parsed structures,
  not byte equality where JSON key order differs.
- A phase is not done until its issues close and the phase gate in
  `docs/04-implementation-plan.md` passes.

## Change rules

- One phase per branch (`phase-N-<slug>`), conventional commit messages,
  PRs use `Closes #N`.
- Every behavior change updates `docs/` in the same PR.
- Never commit generated binaries, logs, or local state files.
