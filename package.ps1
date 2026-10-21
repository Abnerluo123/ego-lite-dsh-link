# Packs full and Ego-native releases with complete, buildable source for both tags.
param(
    [string]$Version = "1.7.0",
    [string]$OutDir = "$PSScriptRoot\dist",
    [string]$BinaryDir = ""
)
$ErrorActionPreference = "Stop"
if ($Version -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]*$') { throw "invalid version: $Version" }
$OutDir = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutDir)
if (-not $BinaryDir) { $BinaryDir = $OutDir }
$BinaryDir = (Resolve-Path -LiteralPath $BinaryDir -ErrorAction Stop).ProviderPath
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$suffixes = @("windows-amd64.exe", "windows-arm64.exe", "darwin-arm64", "darwin-amd64", "linux-amd64", "linux-arm64")
# Validate the entire release before producing either archive. Never silently ship an incomplete set.
foreach ($prefix in @("zero-ego", "zero-ego-lite")) {
    foreach ($suffix in $suffixes) {
        $binary = Join-Path $BinaryDir "$prefix-$suffix"
        if (-not (Test-Path -LiteralPath $binary -PathType Leaf)) { throw "release binary missing: $binary" }
    }
}
$commonFiles = @("AGENTS.md", "install.ps1", "install.sh", "build.ps1", "package.ps1", "go.mod", "LICENSE", "integration_feedback.py")
$sourceFiles = @(Get-ChildItem -LiteralPath $PSScriptRoot -File | Where-Object { $_.Extension -in @(".go", ".js", ".mjs") })
$docFiles = @(Get-ChildItem -LiteralPath (Join-Path $PSScriptRoot "docs") -File -Recurse -ErrorAction Stop)
foreach ($variant in @("full", "lite")) {
    $stage = Join-Path ([System.IO.Path]::GetTempPath()) ("zero-ego-pack-" + [guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $stage | Out-Null
    $ownedStage = (Resolve-Path -LiteralPath $stage).ProviderPath
    try {
        foreach ($file in $commonFiles) { Copy-Item -LiteralPath (Join-Path $PSScriptRoot $file) -Destination $stage }
        foreach ($file in $sourceFiles) { Copy-Item -LiteralPath $file.FullName -Destination $stage }
        foreach ($file in $docFiles) {
            $docsRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "docs")).ProviderPath.TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
            $relative = $file.FullName.Substring($docsRoot.Length)
            $destination = Join-Path (Join-Path $stage "docs") $relative
            $parent = Split-Path -Parent $destination
            New-Item -ItemType Directory -Force -Path $parent | Out-Null
            Copy-Item -LiteralPath $file.FullName -Destination $destination
        }
        # Include installer/release regression tests when present, without shipping the stale Python alternative.
        foreach ($testName in @("test-install-package.ps1", "test-install.sh")) {
            $releaseTests = Join-Path $PSScriptRoot $testName
            if (Test-Path -LiteralPath $releaseTests -PathType Leaf) { Copy-Item -LiteralPath $releaseTests -Destination $stage }
        }
        $readme = if ($variant -eq "lite") { "README.lite.md" } else { "README.md" }
        Copy-Item -LiteralPath (Join-Path $PSScriptRoot $readme) -Destination (Join-Path $stage "README.md")
        # Both readmes let an extracted release be repackaged as either variant.
        Copy-Item -LiteralPath (Join-Path $PSScriptRoot "README.lite.md") -Destination $stage
        $prefix = if ($variant -eq "lite") { "zero-ego-lite" } else { "zero-ego" }
        foreach ($suffix in $suffixes) { Copy-Item -LiteralPath (Join-Path $BinaryDir "$prefix-$suffix") -Destination $stage }
        $zip = Join-Path $OutDir "$prefix-$Version.zip"
        Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zip -CompressionLevel Optimal -Force
        Get-Item -LiteralPath $zip | Select-Object Name, Length
    }
    finally {
        # The stage is unique to this invocation; verify its resolved identity before removing it.
        if ((Test-Path -LiteralPath $stage) -and (Resolve-Path -LiteralPath $stage).ProviderPath -eq $ownedStage) {
            Remove-Item -LiteralPath $ownedStage -Recurse -Force
        }
    }
}
