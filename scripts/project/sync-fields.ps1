#Requires -Version 5.1
<#
.SYNOPSIS
    Sets GitHub Project field values for the DSCodex-Go backlog.

.DESCRIPTION
    Reads scripts/project/issues.json and writes Phase, Area, Platform,
    Priority, Size, and Est. days for every epic and task issue in the
    project. Uses explicit GraphQL node IDs and batches mutations to stay
    inside the GraphQL point budget, and waits for the hourly reset when the
    budget is exhausted.

    Invoked by bootstrap.ps1, but can be run on its own to re-sync fields.

.PARAMETER Owner
    GitHub user or org that owns the repository and project.

.PARAMETER Repo
    Repository name.

.PARAMETER ProjectNumber
    Project number. Resolved by title when omitted.

.PARAMETER BatchSize
    Mutations per GraphQL request. Default 20.
#>
[CmdletBinding()]
param(
    [string]$Owner = "",
    [string]$Repo = "",
    [int]$ProjectNumber = 0,
    [int]$BatchSize = 20
)

$ErrorActionPreference = "Stop"

# gh emits UTF-8; Windows PowerShell 5.1 would otherwise decode native command
# output with the OEM code page and fail to match existing titles.
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

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

function Invoke-GraphQL {
    param([string]$Query)
    # Read the query from stdin: Windows PowerShell 5.1 strips embedded double
    # quotes from native command arguments, which corrupts GraphQL passed via
    # -f query=... The stdin encoding must be UTF-8 without a BOM.
    $previousPreference = $ErrorActionPreference
    $previousEncoding = $OutputEncoding
    $ErrorActionPreference = 'Continue'
    $OutputEncoding = New-Object System.Text.UTF8Encoding($false)
    $raw = $Query | & gh api graphql -F query=@- 2>&1
    $code = $LASTEXITCODE
    $OutputEncoding = $previousEncoding
    $ErrorActionPreference = $previousPreference
    return [pscustomobject]@{ Code = $code; Raw = ($raw -join "`n") }
}

function Test-GraphQLErrors {
    param([string]$Raw)
    try {
        $json = $Raw | ConvertFrom-Json
    } catch {
        return $true
    }
    if ($json.PSObject.Properties.Name -notcontains 'errors') { return $false }
    if ($null -eq $json.errors) { return $false }
    return @($json.errors).Count -gt 0
}

function Get-GraphQLRemaining {
    $raw = & gh api graphql -f "query={ rateLimit { remaining resetAt } }" 2>$null
    try {
        $json = ($raw -join "`n") | ConvertFrom-Json
        return [pscustomobject]@{
            Remaining = [int]$json.data.rateLimit.remaining
            ResetAt   = [DateTime]::Parse($json.data.rateLimit.resetAt).ToUniversalTime()
        }
    } catch {
        return [pscustomobject]@{ Remaining = -1; ResetAt = [DateTime]::UtcNow.AddMinutes(5) }
    }
}

function Wait-ForGraphQLReset {
    param([string]$Reason = "GraphQL rate limit exhausted")
    $state = Get-GraphQLRemaining
    $wait = 60
    if ($state.ResetAt) {
        $wait = [int][math]::Ceiling(($state.ResetAt - [DateTime]::UtcNow).TotalSeconds) + 15
        if ($wait -lt 30) { $wait = 30 }
    }
    Write-Host "  $Reason; waiting $wait s until $($state.ResetAt.ToString('HH:mm:ss')) UTC"
    Start-Sleep -Seconds $wait
}

if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    throw "The gh CLI is required: https://cli.github.com/"
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$issuesFile = Join-Path $scriptDir "issues.json"
$spec = Get-Content -LiteralPath $issuesFile -Raw -Encoding UTF8 | ConvertFrom-Json

if (-not $Owner -or -not $Repo) {
    $nwo = & gh repo view --json nameWithOwner --jq ".nameWithOwner" 2>$null
    if ($LASTEXITCODE -ne 0 -or -not $nwo) {
        throw "Could not detect the repository. Run from the repo root or pass -Owner and -Repo."
    }
    $parts = $nwo.Trim().Split("/")
    if (-not $Owner) { $Owner = $parts[0] }
    if (-not $Repo) { $Repo = $parts[1] }
}

$state = Get-GraphQLRemaining
if ($state.Remaining -eq 0) {
    Wait-ForGraphQLReset -Reason "GraphQL budget exhausted before field sync"
}

if ($ProjectNumber -eq 0) {
    $projects = (Invoke-Gh @("project", "list", "--owner", $Owner, "--limit", "100", "--format", "json") | ConvertFrom-Json).projects
    $match = $projects | Where-Object { $_.title -eq $spec.project.title } | Select-Object -First 1
    if (-not $match) {
        throw "Project '$($spec.project.title)' not found; run bootstrap.ps1 first."
    }
    $ProjectNumber = $match.number
}

Write-Step "Field values (project #$ProjectNumber)"

# Desired values keyed by issue title.
$desired = @{}
foreach ($epic in $spec.epics) {
    $desired[$epic.title] = @{
        phase = $epic.phase; area = $epic.area; platform = $epic.platform
        priority = $epic.priority; size = $epic.size; estDays = [double]$epic.est_days
    }
    foreach ($task in $epic.tasks) {
        $desired[$task.title] = @{
            phase = $epic.phase; area = $task.area; platform = $task.platform
            priority = $task.priority; size = $task.size; estDays = [double]$task.est_days
        }
    }
}

# Issue numbers by title (REST, not GraphQL).
$issues = Invoke-Gh @("issue", "list", "--state", "all", "--limit", "1000", "--json", "number,title") | ConvertFrom-Json
$numberByTitle = @{}
foreach ($issue in $issues) { $numberByTitle[$issue.title] = $issue.number }

# Project node ID and field/option IDs (GraphQL).
$projectId = (Invoke-Gh @("project", "view", "$ProjectNumber", "--owner", $Owner, "--format", "json") | ConvertFrom-Json).id
$fieldList = (Invoke-Gh @("project", "field-list", "$ProjectNumber", "--owner", $Owner, "--format", "json") | ConvertFrom-Json).fields
$fieldId = @{}
$optionId = @{}
foreach ($field in $fieldList) {
    $fieldId[$field.name] = $field.id
    if ($field.options) {
        foreach ($option in $field.options) {
            $optionId["$($field.name)|$($option.name)"] = $option.id
        }
    }
}

# Project item IDs by issue number (GraphQL).
$items = (Invoke-Gh @("project", "item-list", "$ProjectNumber", "--owner", $Owner, "--limit", "1000", "--format", "json") | ConvertFrom-Json).items
$itemIdByNumber = @{}
foreach ($item in $items) {
    if ($item.content -and $item.content.number) {
        $itemIdByNumber[[string]$item.content.number] = $item.id
    }
}

# Build mutations.
$mutations = New-Object System.Collections.Generic.List[string]
$missing = New-Object System.Collections.Generic.List[string]
foreach ($title in $desired.Keys) {
    if (-not $numberByTitle.ContainsKey($title)) { $missing.Add($title); continue }
    $number = [string]$numberByTitle[$title]
    if (-not $itemIdByNumber.ContainsKey($number)) { $missing.Add($title); continue }
    $itemId = $itemIdByNumber[$number]
    $want = $desired[$title]

    $pairs = @(
        @{ field = "Phase"; value = $want.phase },
        @{ field = "Area"; value = $want.area },
        @{ field = "Platform"; value = $want.platform },
        @{ field = "Priority"; value = $want.priority },
        @{ field = "Size"; value = $want.size }
    )
    foreach ($pair in $pairs) {
        $fid = $fieldId[$pair.field]
        $oid = $optionId["$($pair.field)|$($pair.value)"]
        if (-not $fid -or -not $oid) { continue }
        $alias = "m" + $mutations.Count
        $mutations.Add("$alias`: updateProjectV2ItemFieldValue(input:{projectId:`"$projectId`",itemId:`"$itemId`",fieldId:`"$fid`",value:{singleSelectOptionId:`"$oid`"}}){projectV2Item{id}}")
    }

    $estFieldId = $fieldId["Est. days"]
    if ($estFieldId) {
        $estText = ([double]$want.estDays).ToString([System.Globalization.CultureInfo]::InvariantCulture)
        $alias = "m" + $mutations.Count
        $mutations.Add("$alias`: updateProjectV2ItemFieldValue(input:{projectId:`"$projectId`",itemId:`"$itemId`",fieldId:`"$estFieldId`",value:{number:$estText}}){projectV2Item{id}}")
    }
}

if ($missing.Count -gt 0) {
    Write-Warning "$($missing.Count) issues were not found in the repository or project"
}
if ($mutations.Count -eq 0) {
    Write-Host "Nothing to update."
    return
}
Write-Host "Updating $($mutations.Count) field values in batches of $BatchSize"

$total = $mutations.Count
$index = 0
$done = 0
while ($index -lt $total) {
    $batch = New-Object System.Collections.Generic.List[string]
    for ($i = 0; $i -lt $BatchSize -and ($index + $i) -lt $total; $i++) {
        $batch.Add($mutations[$index + $i])
    }
    $query = "mutation { " + ($batch -join " ") + " }"

    $attempt = 0
    while ($true) {
        $attempt++
        $result = Invoke-GraphQL -Query $query
        if ($result.Raw -match 'RATE_LIMIT') {
            if ($attempt -gt 5) { throw "GraphQL rate limit persisted after $attempt attempts" }
            Wait-ForGraphQLReset
            continue
        }
        if (Test-GraphQLErrors -Raw $result.Raw) {
            Write-Warning "Batch starting at index $index returned GraphQL errors: $(($result.Raw -replace '\s+', ' ').Substring(0, [math]::Min(300, $result.Raw.Length)))"
        }
        break
    }

    $index += $batch.Count
    $done += $batch.Count
    Write-Host "  set $done/$total field values"
    Start-Sleep -Milliseconds 400
}

Write-Host "Field values synced."
