# Installs the connector outside the DSH workspace and prints or updates its profile entry.
[CmdletBinding()]
param(
    [string]$EgoCli = "",
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA "ego-dsh-link"),
    [string]$Binary = "",
    [string]$Version = "1.7.1",
    [ValidateSet("auto", "amd64", "arm64")][string]$Architecture = "auto",
    [switch]$ApplyProfile
)

$ErrorActionPreference = "Stop"
if ($Version -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]*$') { throw "invalid version: $Version" }
if ($Architecture -eq "auto") {
    $nativeArch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
    $Architecture = switch ($nativeArch) {
        "ARM64" { "arm64" }
        "AMD64" { "amd64" }
        default { throw "unsupported Windows architecture: $nativeArch (pass -Architecture amd64 or arm64)" }
    }
}
if (-not $Binary) {
    $candidates = @(
        (Join-Path $PSScriptRoot "dist\zero-ego-windows-$Architecture.exe"),
        (Join-Path $PSScriptRoot "zero-ego-windows-$Architecture.exe"),
        (Join-Path $PSScriptRoot "dist\zero-ego-lite-windows-$Architecture.exe"),
        (Join-Path $PSScriptRoot "zero-ego-lite-windows-$Architecture.exe")
    )
    $Binary = $candidates | Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } | Select-Object -First 1
    if (-not $Binary) { throw "connector binary not found for windows/$Architecture (run build.ps1 first)" }
}
$Binary = (Resolve-Path -LiteralPath $Binary -ErrorAction Stop).ProviderPath
if (-not (Test-Path -LiteralPath $Binary -PathType Leaf)) { throw "connector binary not found: $Binary" }
$isLite = (Split-Path $Binary -Leaf) -like "*lite*"
if (-not $EgoCli) {
    $candidates = @(
        (Join-Path $env:LOCALAPPDATA "share\ego\active_version_dir\ego-browser.exe"),
        (Join-Path $env:USERPROFILE ".local\share\ego\active_version_dir\ego-browser.exe")
    )
    $cmd = Get-Command ego-browser -ErrorAction SilentlyContinue
    if ($cmd) { $candidates = @($cmd.Source) + $candidates }
    $EgoCli = $candidates | Where-Object { $_ -and (Test-Path -LiteralPath $_ -PathType Leaf) } | Select-Object -First 1
    if (-not $EgoCli) { throw "ego-browser not found; pass -EgoCli <path>" }
}
$EgoCli = (Resolve-Path -LiteralPath $EgoCli -ErrorAction Stop).ProviderPath
$InstallDir = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($InstallDir)
foreach ($value in @($EgoCli, $InstallDir)) {
    if ($value -match '[\r\n]') { throw "paths must not contain newlines" }
}
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$name = if ($isLite) { "zero-ego-lite-$Version.exe" } else { "zero-ego-$Version.exe" }
$target = Join-Path $InstallDir $name
$sourceHash = (Get-FileHash -LiteralPath $Binary).Hash
if ((Test-Path -LiteralPath $target -PathType Leaf) -and (Get-FileHash -LiteralPath $target).Hash -eq $sourceHash) {
    Write-Host "already installed: $target"
}
else {
    # A unique path also supports rebuilding the same version without stopping a running server.
    if (Test-Path -LiteralPath $target) { $target = Join-Path $InstallDir ($name -replace '\.exe$', "-$($sourceHash.Substring(0,12).ToLowerInvariant()).exe") }
    if (-not (Test-Path -LiteralPath $target -PathType Leaf) -or (Get-FileHash -LiteralPath $target).Hash -ne $sourceHash) {
        Copy-Item -LiteralPath $Binary -Destination $target -Force
    }
    if ((Get-FileHash -LiteralPath $target).Hash -ne $sourceHash) { throw "copy verification failed for $target" }
    Write-Host "installed: $target"
}

function Quote-Yaml([string]$Value) { return "'" + $Value.Replace("\", "/").Replace("'", "''") + "'" }
$commandValue = Quote-Yaml $target
$cwdValue = Quote-Yaml $InstallDir
$cliValue = Quote-Yaml $EgoCli
$block = @"
- insert:
    - id: mcp-ego-lite
      name: "@deepseek-ai/dsh-mcp-client"
      config:
        serverName: ego_lite
        transport: stdio
        command: $commandValue
        cwd: $cwdValue
        env:
          EGO_BROWSER_CLI: $cliValue
        toolCallTimeoutMs: 360000
        failOnStartupError: false
"@
$profileDir = if ($env:DSH_PROFILE_DIR) { $env:DSH_PROFILE_DIR } else { Join-Path $env:USERPROFILE ".dsh\profiles\desktop" }
$patch = Join-Path $profileDir "cordis.patch.yml"
if (-not $ApplyProfile) {
    Write-Host "ego cli: $EgoCli"
    Write-Host "Add this to $patch`n`n$block"
    return
}
if (-not (Test-Path -LiteralPath $patch -PathType Leaf)) { throw "profile patch not found: $patch" }
$original = [System.IO.File]::ReadAllText($patch)
$newline = if ($original.Contains("`r`n")) { "`r`n" } else { "`n" }
$lines = [System.Collections.Generic.List[string]]::new()
$lines.AddRange([string[]]($original -split '\r?\n'))
$matchingEntries = @()
for ($i = 0; $i -lt $lines.Count; $i++) {
    if ($lines[$i] -match '^([ ]*)-\s+id:\s*["'']?mcp-ego-lite["'']?\s*(?:#.*)?$') { $matchingEntries += $i }
}
if ($matchingEntries.Count -gt 1) { throw "multiple mcp-ego-lite entries found; refusing an ambiguous profile update" }
if ($matchingEntries.Count -eq 0) {
    $updated = $original.TrimEnd("`r", "`n") + $newline + $block.Replace("`r`n", "`n").Replace("`n", $newline) + $newline
}
else {
    $begin = $matchingEntries[0]
    $baseIndent = ([regex]::Match($lines[$begin], '^ *')).Length
    $end = $lines.Count
    for ($i = $begin + 1; $i -lt $lines.Count; $i++) {
        if ($lines[$i] -match '^\s*(#.*)?$') { continue }
        if (([regex]::Match($lines[$i], '^ *')).Length -le $baseIndent) { $end = $i; break }
    }
    # The generated installer schema uses block mappings. Refuse flow/scalar config rather than damage it.
    $config = -1
    for ($i = $begin + 1; $i -lt $end; $i++) {
        if ($lines[$i] -match '^([ ]*)config:\s*(?:#.*)?$' -and $Matches[1].Length -eq $baseIndent + 2) { $config = $i; break }
    }
    if ($config -lt 0) { throw "mcp-ego-lite must have a block config mapping" }
    $configIndent = $baseIndent + 2
    $configEnd = $end
    for ($i = $config + 1; $i -lt $end; $i++) {
        if ($lines[$i] -notmatch '^\s*(#.*)?$' -and ([regex]::Match($lines[$i], '^ *')).Length -le $configIndent) { $configEnd = $i; break }
    }
    $entry = [System.Collections.Generic.List[string]]::new()
    $entry.AddRange([string[]]$lines.GetRange($config + 1, $configEnd - $config - 1))
    $fieldIndent = $configIndent + 2
    foreach ($field in @(@("command", $commandValue), @("cwd", $cwdValue))) {
        $found = -1
        for ($i = 0; $i -lt $entry.Count; $i++) {
            if ($entry[$i] -match ('^ {' + $fieldIndent + '}' + $field[0] + ':')) {
                if ($found -ge 0) { throw "duplicate $($field[0]) in mcp-ego-lite config" }
                $found = $i
            }
        }
        $line = (' ' * $fieldIndent) + $field[0] + ': ' + $field[1]
        if ($found -ge 0) { $entry[$found] = $line } else { $entry.Add($line) }
    }
    $envStart = -1
    for ($i = 0; $i -lt $entry.Count; $i++) {
        if ($entry[$i] -match ('^ {' + $fieldIndent + '}env:')) {
            if ($envStart -ge 0) { throw "duplicate env in mcp-ego-lite config" }
            if ($entry[$i] -notmatch ('^ {' + $fieldIndent + '}env:\s*(?:#.*)?$')) { throw "mcp-ego-lite env must be a block mapping" }
            $envStart = $i
        }
    }
    if ($envStart -lt 0) { $entry.Add((' ' * $fieldIndent) + 'env:'); $envStart = $entry.Count - 1 }
    $envEnd = $entry.Count
    for ($i = $envStart + 1; $i -lt $entry.Count; $i++) {
        if ($entry[$i] -notmatch '^\s*(#.*)?$' -and ([regex]::Match($entry[$i], '^ *')).Length -le $fieldIndent) { $envEnd = $i; break }
    }
    $cliIndex = -1
    for ($i = $envStart + 1; $i -lt $envEnd; $i++) {
        if ($entry[$i] -match ('^ {' + ($fieldIndent + 2) + '}EGO_BROWSER_CLI:')) {
            if ($cliIndex -ge 0) { throw "duplicate EGO_BROWSER_CLI in mcp-ego-lite env" }
            $cliIndex = $i
        }
    }
    $cliLine = (' ' * ($fieldIndent + 2)) + 'EGO_BROWSER_CLI: ' + $cliValue
    if ($cliIndex -ge 0) { $entry[$cliIndex] = $cliLine } else { $entry.Insert($envStart + 1, $cliLine) }
    $lines.RemoveRange($config + 1, $configEnd - $config - 1)
    $lines.InsertRange($config + 1, $entry)
    $updated = [string]::Join($newline, $lines)
}
if ($updated -ceq $original) { Write-Host "profile already up to date: $patch"; return }
$backup = "$patch.bak.$(Get-Date -Format 'yyyyMMddHHmmssfff').$([guid]::NewGuid().ToString('N').Substring(0,8))"
$temp = "$patch.tmp.$([guid]::NewGuid().ToString('N'))"
try {
    [System.IO.File]::WriteAllText($temp, $updated, [System.Text.UTF8Encoding]::new($false))
    # File.Replace keeps the existing profile ACL and swaps the complete file atomically.
    [System.IO.File]::Replace($temp, $patch, $backup)
}
finally { if (Test-Path -LiteralPath $temp) { Remove-Item -LiteralPath $temp -Force } }
Write-Host "updated profile: $patch (backup: $backup)"
