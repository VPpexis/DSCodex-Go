#Requires -Version 5.1
<#
.SYNOPSIS
    Enables GitHub secret scanning and push protection for the repository.

.DESCRIPTION
    GitHub blocks pushes containing detected secrets when push protection is
    on. This script sets the repository security settings through the API.
    Requires admin rights on the repository. Secret scanning and push
    protection are free for public repositories; private repositories require
    GitHub Advanced Security.

.PARAMETER Owner
    Repository owner. Defaults to the repository detected by `gh repo view`.

.PARAMETER Repo
    Repository name. Defaults to the repository detected by `gh repo view`.

.EXAMPLE
    pwsh -File scripts/repo/enable-push-protection.ps1
#>
[CmdletBinding()]
param(
    [string]$Owner = "",
    [string]$Repo = ""
)

$ErrorActionPreference = "Stop"

if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    throw "The gh CLI is required: https://cli.github.com/"
}

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
Write-Host "Enabling secret scanning and push protection for $repoFull"

$body = @{
    security_and_analysis = @{
        secret_scanning                     = @{ status = "enabled" }
        secret_scanning_push_protection     = @{ status = "enabled" }
        secret_scanning_non_provider_patterns = @{ status = "enabled" }
        secret_scanning_validity_checks     = @{ status = "enabled" }
    }
} | ConvertTo-Json -Depth 5

$body | & gh api -X PATCH "repos/$repoFull" --input - | Out-Null
if ($LASTEXITCODE -ne 0) {
    Write-Warning "Could not enable all settings."
    Write-Host "  - Admin rights on the repository are required."
    Write-Host "  - Private repositories need GitHub Advanced Security."
    Write-Host "  - Enable manually: Settings -> Code security -> Secret scanning / Push protection"
    exit 1
}

$state = & gh api "repos/$repoFull" --jq ".security_and_analysis"
Write-Host $state
Write-Host "Done. Pushes containing detected secrets will now be blocked."
