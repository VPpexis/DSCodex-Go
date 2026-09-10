#Requires -Version 5.1
<#
.SYNOPSIS
    Idempotent bootstrap of the DSCodex-Go GitHub tracking (labels, milestones,
    epics, task issues, sub-issue links, and the GitHub Project v2 board).

.DESCRIPTION
    Reads scripts/project/issues.json and applies it to the repository through
    the gh CLI. Safe to re-run: existing labels, milestones, issues, fields,
    and project items are reused, never duplicated.

.PARAMETER Owner
    GitHub user or org that owns the repository. Defaults to the repository
    detected by `gh repo view`.

.PARAMETER Repo
    Repository name. Defaults to the repository detected by `gh repo view`.

.PARAMETER SkipProject
    Create/update issues only; do not touch the GitHub Project.

.PARAMETER SkipFields
    Do not set project field values for existing items (faster re-runs).

.EXAMPLE
    gh auth refresh -s project
    pwsh -File scripts/project/bootstrap.ps1

.NOTES
    Requires: gh CLI authenticated with the "project" scope.
#>
[CmdletBinding()]
param(
    [string]$Owner = "",
    [string]$Repo = "",
    [switch]$SkipProject,
    [switch]$SkipFields
)

$ErrorActionPreference = "Stop"

function Write-Step([string]$Message) {
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Invoke-Gh {
    param([string[]]$GhArgs)
    $output = & gh @GhArgs
    if ($LASTEXITCODE -ne 0) {
        throw "gh $($GhArgs -join ' ') failed with exit code $LASTEXITCODE"
    }
    return $output
}

if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    throw "The gh CLI is required: https://cli.github.com/"
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$issuesFile = Join-Path $scriptDir "issues.json"
if (-not (Test-Path -LiteralPath $issuesFile)) {
    throw "Backlog file not found: $issuesFile"
}
$spec = Get-Content -LiteralPath $issuesFile -Raw | ConvertFrom-Json

if (-not $Owner -or -not $Repo) {
    $nwo = & gh repo view --json nameWithOwner --jq ".nameWithOwner" 2>$null
    if ($LASTEXITCODE -ne 0 -or -not $nwo) {
        throw "Could not detect the repository. Run from the repo root or pass -Owner and -Repo."
    }
    $parts = $nwo.Trim().Split("/")
    if (-not $Owner) { $Owner = $parts[0] }
    if (-not $Repo) { $Repo = $parts[1] }
}
$repoFull = "$Owner/$Repo"
Write-Host "Repository: $repoFull"

# ---------------------------------------------------------------------------
# Labels
# ---------------------------------------------------------------------------
Write-Step "Labels"
foreach ($label in $spec.labels) {
    & gh label create $label.name --color $label.color --description $label.description --force | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Write-Warning "Could not create/update label $($label.name)"
    }
}
Write-Host "Labels ensured: $($spec.labels.Count)"

# ---------------------------------------------------------------------------
# Milestones
# ---------------------------------------------------------------------------
Write-Step "Milestones"
$existingMilestones = @()
try {
    $raw = & gh api "repos/$repoFull/milestones?state=all&per_page=100" 2>$null
    if ($LASTEXITCODE -eq 0 -and $raw) { $existingMilestones = $raw | ConvertFrom-Json }
} catch {
    Write-Warning "Could not list milestones; attempting to create them anyway"
}
$milestoneByTitle = @{}
foreach ($m in $existingMilestones) { $milestoneByTitle[$m.title] = $m.number }
foreach ($m in $spec.milestones) {
    if ($milestoneByTitle.ContainsKey($m.title)) {
        Write-Host "  exists: $($m.title)"
        continue
    }
    $created = Invoke-Gh @("api", "-X", "POST", "repos/$repoFull/milestones", "-f", "title=$($m.title)", "-f", "description=$($m.description)") | ConvertFrom-Json
    $milestoneByTitle[$created.title] = $created.number
    Write-Host "  created: $($m.title)"
}

# ---------------------------------------------------------------------------
# Issues (epics and tasks)
# ---------------------------------------------------------------------------
Write-Step "Issues"
$existingIssues = @()
try {
    $raw = & gh issue list --state all --limit 1000 --json number,title,id 2>$null
    if ($LASTEXITCODE -eq 0 -and $raw) { $existingIssues = $raw | ConvertFrom-Json }
} catch {
    Write-Warning "Could not list issues; continuing"
}
$issueByTitle = @{}
foreach ($i in $existingIssues) { $issueByTitle[$i.title] = $i }

function New-IssueIfMissing {
    param(
        [string]$Title,
        [string]$Body,
        [string[]]$Labels,
        [string]$Milestone
    )
    if ($issueByTitle.ContainsKey($Title)) {
        return $issueByTitle[$Title]
    }
    $cmdArgs = @("issue", "create", "--title", $Title, "--body", $Body)
    foreach ($l in $Labels) { $cmdArgs += @("--label", $l) }
    if ($Milestone) { $cmdArgs += @("--milestone", $Milestone) }
    $url = Invoke-Gh $cmdArgs
    $number = [int]($url.Trim().Split("/")[-1])
    $nodeId = (Invoke-Gh @("issue", "view", "$number", "--json", "id", "--jq", ".id")).Trim()
    $entry = [pscustomobject]@{ number = $number; title = $Title; id = $nodeId }
    $issueByTitle[$Title] = $entry
    Write-Host "  created #$number $Title"
    return $entry
}

$epicEntries = @{}
$taskEntries = @()
foreach ($epic in $spec.epics) {
    $epicEntry = New-IssueIfMissing -Title $epic.title -Body $epic.body `
        -Labels @("type/epic", $epic.label) -Milestone $epic.milestone
    $epicEntries[$epic.key] = $epicEntry
    foreach ($task in $epic.tasks) {
        $taskEntry = New-IssueIfMissing -Title $task.title -Body $task.body `
            -Labels @("type/$($task.type)", $epic.label) -Milestone $epic.milestone
        $taskEntries += [pscustomobject]@{
            entry = $taskEntry
            epicKey = $epic.key
            epicNumber = $epicEntry.number
            epicNodeId = $epicEntry.id
            task = $task
            epic = $epic
        }
    }
}
Write-Host "Epics: $($epicEntries.Count); tasks: $($taskEntries.Count)"

# ---------------------------------------------------------------------------
# Sub-issue links
# ---------------------------------------------------------------------------
Write-Step "Sub-issue links"
$subIssueQuery = 'mutation($epic:ID!,$task:ID!){addSubIssue(input:{issueId:$epic,subIssueId:$task}){issue{id}}}'
$linked = 0
$linkSkipped = 0
foreach ($item in $taskEntries) {
    if (-not $item.entry.id -or -not $item.epicNodeId) { $linkSkipped++; continue }
    $null = & gh api graphql -f "query=$subIssueQuery" -f "epic=$($item.epicNodeId)" -f "task=$($item.entry.id)" 2>$null
    if ($LASTEXITCODE -eq 0) {
        $linked++
    } else {
        $linkSkipped++
    }
}
Write-Host "Linked: $linked; skipped (already linked or unavailable): $linkSkipped"

if ($SkipProject) {
    Write-Host ""
    Write-Host "Skipped the GitHub Project (-SkipProject)." -ForegroundColor Yellow
    exit 0
}

# ---------------------------------------------------------------------------
# Project v2
# ---------------------------------------------------------------------------
Write-Step "Project"
$projectsRaw = & gh project list --owner $Owner --limit 100 --format json 2>$null
if ($LASTEXITCODE -ne 0 -or -not $projectsRaw) {
    throw "Could not list projects. Run: gh auth refresh -s project"
}
$projects = ($projectsRaw | ConvertFrom-Json).projects
$project = $projects | Where-Object { $_.title -eq $spec.project.title } | Select-Object -First 1
if ($project) {
    $projectNumber = $project.number
    Write-Host "  exists: $($spec.project.title) (#$projectNumber)"
} else {
    $created = Invoke-Gh @("project", "create", "--owner", $Owner, "--title", $spec.project.title, "--format", "json") | ConvertFrom-Json
    $projectNumber = $created.number
    Write-Host "  created: $($spec.project.title) (#$projectNumber)"
}

$fieldsRaw = & gh project field-list $projectNumber --owner $Owner --format json 2>$null
if ($LASTEXITCODE -ne 0 -or -not $fieldsRaw) {
    throw "Could not list project fields."
}
$fieldNames = @{}
foreach ($f in ($fieldsRaw | ConvertFrom-Json).fields) { $fieldNames[$f.name] = $true }

foreach ($name in $spec.project.fields.PSObject.Properties.Name) {
    if ($fieldNames.ContainsKey($name)) { continue }
    $options = @($spec.project.fields.$name)
    $null = Invoke-Gh @("project", "field-create", "$projectNumber", "--owner", $Owner, `
        "--name", $name, "--data-type", "SINGLE_SELECT", "--single-select-options", ($options -join ","))
    Write-Host "  field created: $name"
}
foreach ($name in @($spec.project.numberFields)) {
    if ($fieldNames.ContainsKey($name)) { continue }
    $null = Invoke-Gh @("project", "field-create", "$projectNumber", "--owner", $Owner, `
        "--name", $name, "--data-type", "NUMBER")
    Write-Host "  field created: $name"
}
foreach ($name in @($spec.project.dateFields)) {
    if ($fieldNames.ContainsKey($name)) { continue }
    $null = Invoke-Gh @("project", "field-create", "$projectNumber", "--owner", $Owner, `
        "--name", $name, "--data-type", "DATE")
    Write-Host "  field created: $name"
}

# ---------------------------------------------------------------------------
# Project items and field values
# ---------------------------------------------------------------------------
Write-Step "Project items"
$issueUrl = { param($number) "https://github.com/$repoFull/issues/$number" }

$allItems = @()
foreach ($epic in $spec.epics) {
    $allItems += [pscustomobject]@{
        number = $epicEntries[$epic.key].number
        phase = $epic.phase
        area = $epic.area
        platform = $epic.platform
        priority = $epic.priority
        size = $epic.size
        estDays = $epic.est_days
    }
    foreach ($task in $epic.tasks) {
        $entry = $issueByTitle[$task.title]
        $allItems += [pscustomobject]@{
            number = $entry.number
            phase = $epic.phase
            area = $task.area
            platform = $task.platform
            priority = $task.priority
            size = $task.size
            estDays = $task.est_days
        }
    }
}

$added = 0
foreach ($item in $allItems) {
    $url = & $issueUrl $item.number
    $null = & gh project item-add $projectNumber --owner $Owner --url $url --format json 2>$null
    if ($LASTEXITCODE -eq 0) { $added++ } else { Write-Warning "Could not add #$($item.number) to the project" }
}
Write-Host "Items added or already present: $added/$($allItems.Count)"

if (-not $SkipFields) {
    Write-Step "Field values (this takes a few minutes)"
    foreach ($item in $allItems) {
        $url = & $issueUrl $item.number
        $edits = @(
            @{ field = "Phase"; value = $item.phase },
            @{ field = "Area"; value = $item.area },
            @{ field = "Platform"; value = $item.platform },
            @{ field = "Priority"; value = $item.priority },
            @{ field = "Size"; value = $item.size }
        )
        foreach ($edit in $edits) {
            $null = & gh project item-edit $projectNumber --owner $Owner --url $url --field $edit.field --value $edit.value 2>$null
            if ($LASTEXITCODE -ne 0) {
                Write-Warning "Could not set $($edit.field) on #$($item.number)"
            }
        }
        $null = & gh project item-edit $projectNumber --owner $Owner --url $url --field "Est. days" --number $item.estDays 2>$null
        if ($LASTEXITCODE -ne 0) {
            Write-Warning "Could not set Est. days on #$($item.number)"
        }
    }
    Write-Host "Field values set."
}

Write-Step "Done"
Write-Host "Project: https://github.com/users/$Owner/projects/$projectNumber"
Write-Host ""
Write-Host "Manual one-time UI steps (see docs/11-project-tracking.md):" -ForegroundColor Yellow
Write-Host "  1. Project Settings -> Status: add 'Blocked' and 'In Review' options"
Write-Host "  2. Create views: Board (Status), Roadmap (Target, group Phase), Phase Table,"
Write-Host "     Windows Track (platform:Windows,All), Docs & Release (area:Docs,CI/Release), Blocked"
Write-Host "  3. Enable workflows: auto-add items, item closed -> Done, item reopened -> Todo,"
Write-Host "     auto-add sub-issues"
