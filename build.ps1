# Builds both connector variants for every supported platform (Go 1.22+).
param(
    [string]$GoExe = "go",
    [string]$OutDir = "$PSScriptRoot\dist"
)
$ErrorActionPreference = "Stop"
$OutDir = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutDir)
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$environmentNames = @("GOOS", "GOARCH", "CGO_ENABLED", "GOCACHE", "GOTMPDIR")
$savedEnvironment = @{}
foreach ($key in $environmentNames) { $savedEnvironment[$key] = [Environment]::GetEnvironmentVariable($key, "Process") }
$targets = @(
    @{ GOOS = "windows"; GOARCH = "amd64" },
    @{ GOOS = "windows"; GOARCH = "arm64" },
    @{ GOOS = "darwin"; GOARCH = "arm64" },
    @{ GOOS = "darwin"; GOARCH = "amd64" },
    @{ GOOS = "linux"; GOARCH = "amd64" },
    @{ GOOS = "linux"; GOARCH = "arm64" }
)
Push-Location $PSScriptRoot
try {
    if (-not $env:GOCACHE) { $env:GOCACHE = Join-Path $PSScriptRoot ".gocache" }
    if (-not $env:GOTMPDIR) { $env:GOTMPDIR = Join-Path $PSScriptRoot ".gotmp" }
    New-Item -ItemType Directory -Force -Path $env:GOCACHE, $env:GOTMPDIR | Out-Null
    $env:CGO_ENABLED = "0"
    # Tests must run on the host, even when the caller has cross-compilation variables set.
    Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
    & $GoExe test ./...
    if ($LASTEXITCODE -ne 0) { throw "go test failed" }
    & $GoExe test -tags egoonly ./...
    if ($LASTEXITCODE -ne 0) { throw "go test -tags egoonly failed" }
    foreach ($target in $targets) {
        $env:GOOS = $target.GOOS
        $env:GOARCH = $target.GOARCH
        $extension = if ($target.GOOS -eq "windows") { ".exe" } else { "" }
        foreach ($variant in @("full", "lite")) {
            $prefix = if ($variant -eq "lite") { "zero-ego-lite" } else { "zero-ego" }
            $name = "$prefix-$($target.GOOS)-$($target.GOARCH)$extension"
            $arguments = @("build", "-trimpath")
            if ($variant -eq "lite") { $arguments += @("-tags", "egoonly") }
            $arguments += @("-ldflags", "-s -w", "-o", (Join-Path $OutDir $name), ".")
            & $GoExe @arguments
            if ($LASTEXITCODE -ne 0) { throw "go build failed for $name" }
            Write-Host "built $name"
        }
    }
}
finally {
    Pop-Location
    foreach ($key in $environmentNames) { [Environment]::SetEnvironmentVariable($key, $savedEnvironment[$key], "Process") }
}
