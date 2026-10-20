# Installs the connector on this machine and prints the DSH profile entry.
#
# The binary is copied OUTSIDE the DSH workspace on purpose: DSH's Windows
# sandbox blocks the Ego bootstrap IPC for executables that live under the
# workspace, which surfaces as "Failed to connect to ego_cli bootstrap".
param(
    [string]$EgoCli = "",
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA "ego-dsh-link"),
    [string]$Binary = "",
    [string]$Version = "1.7.0",
    [switch]$ApplyProfile
)

$ErrorActionPreference = "Stop"

if (-not $Binary) {
    $Binary = Join-Path $PSScriptRoot "dist\ego-browser-dsh-windows-amd64.exe"
}
if (-not (Test-Path $Binary)) {
    throw "connector binary not found: $Binary (run build.ps1 first)"
}

if (-not $EgoCli) {
    $candidates = @(
        (Join-Path $env:LOCALAPPDATA "..\Local\share\ego\active_version_dir\ego-browser.exe"),
        (Join-Path $env:USERPROFILE ".local\share\ego\active_version_dir\ego-browser.exe")
    )
    $cmd = Get-Command ego-browser -ErrorAction SilentlyContinue
    if ($cmd) { $candidates = @($cmd.Source) + $candidates }
    $EgoCli = $candidates | Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1
    if (-not $EgoCli) { throw "ego-browser not found; pass -EgoCli <path>" }
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
# Versioned file name: overwriting a running image fails on Windows with a lock,
# and the copy would silently keep the old build. A new name sidesteps the lock
# and makes it obvious which build is installed.
$target = Join-Path $InstallDir "ego-browser-dsh-$Version.exe"

Get-Process -Name "ego-browser-dsh" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Copy-Item $Binary $target -Force
$sourceHash = (Get-FileHash $Binary).Hash
$targetHash = (Get-FileHash $target).Hash
if ($sourceHash -ne $targetHash) { throw "copy verification failed for $target" }

Write-Host "installed: $target"
Write-Host "ego cli  : $($EgoCli -replace '\\','/')"

$block = @"
- insert:
    - id: mcp-ego-lite
      name: "@deepseek-ai/dsh-mcp-client"
      config:
        serverName: ego_lite
        transport: stdio
        command: "$($target -replace '\\','/')"
        cwd: "$($InstallDir -replace '\\','/')"
        env:
          EGO_BROWSER_CLI: "$($EgoCli -replace '\\','/')"
        toolCallTimeoutMs: 180000
        failOnStartupError: false
"@

$profileDir = if ($env:DSH_PROFILE_DIR) { $env:DSH_PROFILE_DIR } else { Join-Path $env:USERPROFILE ".dsh\profiles\desktop" }
$patch = Join-Path $profileDir "cordis.patch.yml"

if ($ApplyProfile) {
    if (-not (Test-Path $patch)) { throw "profile patch not found: $patch" }
    if ((Get-Content $patch -Raw) -match "mcp-ego-lite") {
        Write-Host "profile already contains mcp-ego-lite; left unchanged"
    }
    else {
        Add-Content -Path $patch -Value "`n$block" -Encoding utf8
        Write-Host "appended mcp-ego-lite to $patch"
    }
}
else {
    Write-Host ""
    Write-Host "Add this to $patch"
    Write-Host ""
    Write-Host $block
}
