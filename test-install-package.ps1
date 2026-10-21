# Temporary-directory integration tests. Never reads or writes the real DSH profile.
param([string]$GoExe = "go", [string]$BashExe = "", [string]$TempDir = "$PSScriptRoot\.gotmp", [switch]$KeepTemp)
$ErrorActionPreference = "Stop"
function Assert([bool]$Condition, [string]$Message) { if (-not $Condition) { throw "assertion failed: $Message" } }
function Put([string]$Path, [string]$Text) { [System.IO.File]::WriteAllText($Path, $Text, [System.Text.UTF8Encoding]::new($false)) }
function ReadText([string]$Path) { return [System.IO.File]::ReadAllText($Path) }
New-Item -ItemType Directory -Path $TempDir -Force | Out-Null
$testRoot = Join-Path $TempDir ("zero-ego-release-test-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $testRoot | Out-Null
$ownedRoot = (Resolve-Path -LiteralPath $testRoot).ProviderPath
$envKeys = @("DSH_PROFILE_DIR", "PROCESSOR_ARCHITECTURE", "PROCESSOR_ARCHITEW6432", "GOCACHE", "GOTMPDIR", "GOOS", "GOARCH", "CGO_ENABLED")
$saved = @{}
foreach ($key in $envKeys) { $saved[$key] = [Environment]::GetEnvironmentVariable($key, "Process") }
try {
    $fixture = Join-Path $testRoot "source"
    $profile = Join-Path $testRoot "profile"
    $installDir = Join-Path $testRoot ("installed O'Brien " + [char]0x4E2D + [char]0x6587)
    New-Item -ItemType Directory -Path $fixture, $profile, (Join-Path $fixture "dist") | Out-Null
    foreach ($name in @("install.ps1", "install.sh", "test-install.sh", "package.ps1", "build.ps1", "README.md", "README.lite.md", "AGENTS.md", "go.mod", "LICENSE", "integration_feedback.py")) {
        Copy-Item -LiteralPath (Join-Path $PSScriptRoot $name) -Destination $fixture
    }
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot "docs") -Destination $fixture -Recurse
    foreach ($file in Get-ChildItem -LiteralPath $PSScriptRoot -File | Where-Object { $_.Extension -in @(".go", ".js", ".mjs") }) { Copy-Item -LiteralPath $file.FullName -Destination $fixture }
    Put (Join-Path $fixture "future_source_test.go") "package main`nvar futurePackagedSource = true`n"
    Put (Join-Path $fixture "future_browser_helper.js") "// future helper inclusion sentinel`n"
    $suffixes = @("windows-amd64.exe", "windows-arm64.exe", "darwin-arm64", "darwin-amd64", "linux-amd64", "linux-arm64")
    foreach ($prefix in @("zero-ego", "zero-ego-lite")) {
        foreach ($suffix in $suffixes) { Put (Join-Path $fixture "dist/$prefix-$suffix") "$prefix-$suffix" }
    }
    $fakeCli = Join-Path $testRoot "ego O'Brien.exe"
    Put $fakeCli "not executed"
    $env:DSH_PROFILE_DIR = $profile
    $env:PROCESSOR_ARCHITECTURE = "ARM64"
    Remove-Item Env:PROCESSOR_ARCHITEW6432 -ErrorAction SilentlyContinue
    $patch = Join-Path $profile "cordis.patch.yml"
    $commentOnly = "# mcp-ego-lite in a comment must not prevent installation`n- insert: []`n"
    Put $patch $commentOnly
    & (Join-Path $fixture "install.ps1") -EgoCli $fakeCli -InstallDir $installDir -ApplyProfile
    $installed = Join-Path $installDir "zero-ego-1.6.0.exe"
    Assert ((ReadText $installed) -eq "zero-ego-windows-arm64.exe") "native ARM64 binary selected"
    $after = ReadText $patch
    Assert ($after.Contains("- id: mcp-ego-lite")) "comment-only marker does not suppress insertion"
    Assert ($after.Contains("O''Brien")) "YAML apostrophes escaped"
    Assert ((@(Get-ChildItem -LiteralPath $profile -Filter "*.bak.*")).Count -eq 1) "initial update backed up"
    Assert ((ReadText (@(Get-ChildItem -LiteralPath $profile -Filter "*.bak.*")[0].FullName)) -ceq $commentOnly) "backup is byte-equivalent text"
    & (Join-Path $fixture "install.ps1") -EgoCli $fakeCli -InstallDir $installDir -ApplyProfile
    Assert ((ReadText $patch) -ceq $after) "reinstall profile update is idempotent"
    Assert ((@(Get-ChildItem -LiteralPath $profile -Filter "*.bak.*")).Count -eq 1) "idempotent reinstall avoids extra backup"

    $existing = @'
# preserve comments and unrelated configuration
- insert:
    - id: other
      config:
        command: keep-other
    - id: 'mcp-ego-lite' # exact target
      name: "@deepseek-ai/dsh-mcp-client"
      config:
        command: old-command
        cwd: old-directory
        env:
          EGO_BROWSER_CLI: old-cli
          KEEP_ME: unchanged
        toolCallTimeoutMs: 456789
        customSetting: retained
    - id: other-after
      config:
        command: keep-after
'@
    Put $patch ($existing.Replace("`r`n", "`n").Replace("`n", "`r`n") + "`r`n")
    $beforeUpgrade = ReadText $patch
    & (Join-Path $fixture "install.ps1") -EgoCli $fakeCli -InstallDir $installDir -Version 1.6.1 -Architecture amd64 -ApplyProfile
    $upgraded = ReadText $patch
    Assert ($upgraded.Contains("zero-ego-1.6.1.exe")) "existing command upgraded"
    Assert ((ReadText (Join-Path $installDir "zero-ego-1.6.1.exe")) -eq "zero-ego-windows-amd64.exe") "explicit AMD64 override honored"
    Assert ($upgraded.Contains("KEEP_ME: unchanged") -and $upgraded.Contains("customSetting: retained") -and $upgraded.Contains("toolCallTimeoutMs: 456789")) "custom config and env preserved"
    Assert ($upgraded.Contains("command: keep-other") -and $upgraded.Contains("command: keep-after")) "unrelated entries preserved"
    Assert (-not $upgraded.Contains("old-command") -and -not $upgraded.Contains("old-directory") -and -not $upgraded.Contains("old-cli")) "all three target fields updated"
    Assert (-not [regex]::IsMatch($upgraded, '(?<!\r)\n')) "Windows profile line endings preserved"
    Assert (@(Get-ChildItem -LiteralPath $profile -Filter "*.bak.*" | Where-Object { (ReadText $_.FullName) -ceq $beforeUpgrade }).Count -eq 1) "upgrade backup preserves prior profile"
    $originalImage = ReadText (Join-Path $installDir "zero-ego-1.6.1.exe")
    Put (Join-Path $fixture "dist/zero-ego-windows-amd64.exe") "rebuilt same version"
    & (Join-Path $fixture "install.ps1") -EgoCli $fakeCli -InstallDir $installDir -Version 1.6.1 -Architecture amd64 -ApplyProfile
    Assert ((ReadText (Join-Path $installDir "zero-ego-1.6.1.exe")) -ceq $originalImage) "same-version rebuild leaves old executable intact"
    Assert ((ReadText $patch) -match 'zero-ego-1\.6\.1-[a-f0-9]{12}\.exe') "same-version rebuild selects unique executable"
    & (Join-Path $fixture "install.ps1") -EgoCli $fakeCli -InstallDir $installDir -Binary (Join-Path $fixture "dist/zero-ego-lite-windows-arm64.exe") -ApplyProfile
    Assert ((ReadText $patch).Contains("zero-ego-lite-1.6.0.exe")) "full-to-lite profile switch updates executable"

    Put $patch "- insert:`n    - id: mcp-ego-lite`n      config:`n        serverName: ego_lite`n"
    & (Join-Path $fixture "install.ps1") -EgoCli $fakeCli -InstallDir $installDir -Architecture arm64 -ApplyProfile
    $addedFields = ReadText $patch
    Assert ($addedFields.Contains("        command:") -and $addedFields.Contains("        cwd:") -and $addedFields.Contains("        env:`n          EGO_BROWSER_CLI:")) "missing config fields inserted"
    $ambiguous = "- insert:`n    - id: mcp-ego-lite`n      config:`n        command: one`n    - id: mcp-ego-lite`n      config:`n        command: two`n"
    Put $patch $ambiguous
    $rejected = $false
    try { & (Join-Path $fixture "install.ps1") -EgoCli $fakeCli -InstallDir $installDir -ApplyProfile } catch { $rejected = $true }
    Assert $rejected "duplicate connector entries rejected"
    Assert ((ReadText $patch) -ceq $ambiguous) "ambiguous profile not modified"
    foreach ($script in @("install.ps1", "install.sh")) {
        Assert (-not ((ReadText (Join-Path $fixture $script)) -match 'Stop-Process|Get-Process|\bpkill\b|\bkillall\b')) "installer has no process-wide kill commands"
    }
    Write-Host "PASS: Windows installer isolation, backup, upgrade, architecture and profile preservation"
    if ($BashExe) {
        $savedPath = $env:PATH
        try {
            $env:PATH = (Split-Path -Parent $BashExe) + [System.IO.Path]::PathSeparator + $savedPath
            & $BashExe (Join-Path $fixture "test-install.sh").Replace('\', '/') $fixture.Replace('\', '/')
            if ($LASTEXITCODE -ne 0) { throw "Unix installer integration tests failed" }
        }
        finally { $env:PATH = $savedPath }
    }

    # Package only temporary binary fixtures; clean extraction builds validate source completeness.
    $archives = Join-Path $testRoot "archives"
    & (Join-Path $fixture "package.ps1") -BinaryDir (Join-Path $fixture "dist") -OutDir $archives
    $env:GOCACHE = if ($saved["GOCACHE"]) { $saved["GOCACHE"] } else { Join-Path $PSScriptRoot ".gocache" }
    $env:GOTMPDIR = Join-Path $testRoot "gotmp"
    New-Item -ItemType Directory -Path $env:GOTMPDIR -Force | Out-Null
    Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
    foreach ($prefix in @("zero-ego", "zero-ego-lite")) {
        $extracted = Join-Path $testRoot "$prefix-extracted"
        Expand-Archive -LiteralPath (Join-Path $archives "$prefix-1.6.0.zip") -DestinationPath $extracted
        Assert (-not (Test-Path -LiteralPath (Join-Path $extracted "python"))) "stale Python excluded"
        foreach ($file in Get-ChildItem -LiteralPath $fixture -File | Where-Object { $_.Extension -in @(".go", ".js", ".mjs") }) {
            Assert (Test-Path -LiteralPath (Join-Path $extracted $file.Name)) "source/helper $($file.Name) included"
        }
        Assert (Test-Path -LiteralPath (Join-Path $extracted "build.ps1")) "build script included"
        Assert (Test-Path -LiteralPath (Join-Path $extracted "integration_feedback.py")) "integration feedback source included"
        Assert (Test-Path -LiteralPath (Join-Path $extracted "docs/AGENT_GUIDE.md")) "docs tree included"
        Push-Location $extracted
        try {
            & $GoExe test -count=1 ./...
            if ($LASTEXITCODE -ne 0) { throw "extracted $prefix default-tag tests failed" }
            & $GoExe test -count=1 -tags egoonly ./...
            if ($LASTEXITCODE -ne 0) { throw "extracted $prefix egoonly tests failed" }
            & $GoExe build -o (Join-Path $testRoot "$prefix-default.exe") .
            if ($LASTEXITCODE -ne 0) { throw "extracted $prefix default build failed" }
            & $GoExe build -tags egoonly -o (Join-Path $testRoot "$prefix-egoonly.exe") .
            if ($LASTEXITCODE -ne 0) { throw "extracted $prefix egoonly build failed" }
        }
        finally { Pop-Location }
        Write-Host "PASS: $prefix clean extraction, complete sources/helpers, both-tag tests/builds"
    }
    $missingBinary = Join-Path $fixture "dist/zero-ego-linux-arm64"
    # The target is an owned fixture whose identity was established above.
    Assert ($missingBinary.StartsWith($ownedRoot + [System.IO.Path]::DirectorySeparatorChar)) "delete confined to fixture"
    Remove-Item -LiteralPath $missingBinary
    $rejected = $false
    try { & (Join-Path $fixture "package.ps1") -BinaryDir (Join-Path $fixture "dist") -OutDir (Join-Path $testRoot "incomplete") } catch { $rejected = $true }
    Assert $rejected "incomplete release rejected"
    Write-Host "PASS: incomplete release validation"
}
finally {
    foreach ($key in $envKeys) { [Environment]::SetEnvironmentVariable($key, $saved[$key], "Process") }
    if ($KeepTemp) { Write-Host "test artifacts retained: $ownedRoot" }
    elseif ((Test-Path -LiteralPath $testRoot) -and (Resolve-Path -LiteralPath $testRoot).ProviderPath -eq $ownedRoot) { Remove-Item -LiteralPath $ownedRoot -Recurse -Force }
}
