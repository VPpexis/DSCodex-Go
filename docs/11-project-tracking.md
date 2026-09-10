# 11 — Project Tracking

All port work is tracked in a GitHub Project (v2) linked to this repository.
The backlog is defined as code in `scripts/project/issues.json` and applied by
`scripts/project/bootstrap.ps1`.

## Project

**Title:** `DSCodex-Go Port Tracker` —
<https://github.com/users/VPpexis/projects/2>

### Custom fields

| Field | Type | Values |
| --- | --- | --- |
| Status | built-in single select | Todo · In Progress · Blocked · In Review · Done |
| Phase | single select | P0 Scaffold · P1 Foundation · P2 Router Core · P3 Vision & Compaction · P4 CLI & Lifecycle · P5 Autostart & Supervisor · P6 Bridge · P7 Parity & Differential · P8 Release |
| Area | single select | Router · Vision · Compaction · CLI · Config · Catalog · Keystore · Proxy · Autostart · Bridge · Docs · CI/Release · Testing |
| Platform | single select | All · macOS · Linux · Windows |
| Priority | single select | Critical · High · Medium · Low |
| Size | single select | XS · S · M · L · XL |
| Est. days | number | — |
| Target | date | — |

> GitHub's built-in Status field ships with `Todo`, `In Progress`, and `Done`.
> Add `Blocked` and `In Review` once in the Project UI (Settings → Status →
> edit options) — the CLI cannot edit select options.

### Views (one-time UI setup)

| View | Layout | Configuration |
| --- | --- | --- |
| Board | Board | Group by `Status` |
| Roadmap | Roadmap | Date: `Target`; group by `Phase` |
| Phase Table | Table | Group by `Phase`; show all fields |
| Windows Track | Table | Filter: `platform:Windows,All` |
| Docs & Release | Table | Filter: `area:Docs,CI/Release` |
| Blocked | Table | Filter: `status:Blocked` |

### Creating views (one-time UI)

1. Open the project and click **New view** to the right of the existing view tabs.
2. Rename it: **View options** (gear icon next to the search bar) → **Rename view**.
3. Set the layout: **View options** → **Layout** → Table / Board / Roadmap.
4. Group it: **View options** → **Group by** → choose a field. On a Board, the
   group field becomes the columns.
5. Filter it: click the **Filter** icon in the toolbar, type the filter, press
   Enter. Values with special characters can be picked from the autocomplete.
6. Changes save automatically; if a dot appears next to the view name, choose
   **Save changes** in the View options menu.

The Roadmap layout needs dates: click **Date fields** (top right) and set
**Target date** to `Target`. Items without a Target date appear under
"No date" until populated.

### Built-in workflows (one-time UI setup)

- **Auto-add to project:** all items in this repository.
- **Item closed:** set `Status` → `Done`.
- **Item reopened:** set `Status` → `Todo`.
- **Auto-add sub-issues to project:** on.

## Labels

| Label | Purpose |
| --- | --- |
| `type/epic` | Phase epics |
| `type/feature` | Feature implementation |
| `type/test` | Testing and verification |
| `type/docs` | Documentation |
| `type/chore` | Tooling and maintenance |
| `type/bug` | Something is broken |
| `phase/0-scaffold` … `phase/8-release` | Phase filtering in the issue list |
| `upstream-sync` | Tracked against upstream DSCodex changes |

Area, Platform, Priority, and Size live in project fields only, to avoid
double bookkeeping.

## Milestones

| Milestone | Phases |
| --- | --- |
| v0.1.0 — Foundation | P0, P1 |
| v0.2.0 — Router Core | P2, P3 |
| v0.5.0 — CLI & Platform | P4, P5 |
| v0.9.0 — Bridge & Parity | P6, P7 |
| v1.0.0 — Parity Release | P8 |

## Issue hierarchy

- One epic per phase (P0–P8) with the phase gate as a checklist.
- Task issues are sub-issues of their epic; epic progress shows in the Project.
- Each task issue lists its upstream reference and acceptance criteria.

## Bootstrap

```powershell
# once per machine
gh auth refresh -s project

# from the repository root (idempotent; safe to re-run)
pwsh -File scripts/project/bootstrap.ps1
# or Windows PowerShell:
powershell -ExecutionPolicy Bypass -File scripts/project/bootstrap.ps1
```

The script creates or updates labels, milestones, epics, and task issues from
`scripts/project/issues.json`, links sub-issues, creates the Project, ensures
fields, and sets field values. It prints the remaining manual UI steps.

Field values are written by `scripts/project/sync-fields.ps1`, which
`bootstrap.ps1` invokes. It batches GraphQL mutations and waits for the hourly
reset when GitHub's 5,000-point GraphQL budget is exhausted, so a run can take
up to an hour if the budget is already spent. Re-run it any time to re-sync:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/project/sync-fields.ps1
```

To change the backlog, edit `issues.json` and re-run the script.

## Status flow

```
Todo ──branch created──▶ In Progress ──PR opened──▶ In Review ──merged + gate──▶ Done
             ▲                                          │
             └──────────── changes requested ◀──────────┘
Blocked: waiting on an external dependency (comment explains)
```

- Work items move to **In Progress** when the phase branch is cut.
- PRs reference their issue with `Closes #N`; merging closes the issue and the
  workflow moves it to **Done**.
- A phase epic closes only when all sub-issues are Done and the gate checklist
  in `docs/04-implementation-plan.md` passes.

## Definition of Done (per task)

1. Behavior matches the upstream reference (or a divergence is recorded in
   `docs/03-parity-matrix.md`).
2. Tests added or updated; `go test -race ./...` green.
3. `gofmt` and `go vet` clean.
4. Documentation updated in the same PR.
5. No secrets, keys, or proxy credentials anywhere in code, logs, or fixtures.
