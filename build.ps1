# Builds the connector for every supported platform.
# Requires Go 1.22+ on PATH (or pass -GoExe).
param(
    [string]$GoExe = "go",
    [string]$OutDir = "$PSScriptRoot\dist"
)

$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

# Some locked-down Windows profiles deny %LOCALAPPDATA%\go-build. Keep the build
# cache and temp files beside the project unless the caller set them.
if (-not $env:GOCACHE) { $env:GOCACHE = Join-Path $PSScriptRoot ".gocache" }
if (-not $env:GOTMPDIR) { $env:GOTMPDIR = Join-Path $PSScriptRoot ".gotmp" }
New-Item -ItemType Directory -Force -Path $env:GOCACHE, $env:GOTMPDIR | Out-Null

$env:CGO_ENABLED = "0"
$targets = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; Name = "ego-browser-dsh-windows-amd64.exe" },
    @{ GOOS = "windows"; GOARCH = "arm64"; Name = "ego-browser-dsh-windows-arm64.exe" },
    @{ GOOS = "darwin";  GOARCH = "arm64"; Name = "ego-browser-dsh-darwin-arm64" },
    @{ GOOS = "darwin";  GOARCH = "amd64"; Name = "ego-browser-dsh-darwin-amd64" },
    @{ GOOS = "linux";   GOARCH = "amd64"; Name = "ego-browser-dsh-linux-amd64" },
    @{ GOOS = "linux";   GOARCH = "arm64"; Name = "ego-browser-dsh-linux-arm64" }
)

Push-Location $PSScriptRoot
try {
    & $GoExe test ./...
    if ($LASTEXITCODE -ne 0) { throw "go test failed" }
    foreach ($target in $targets) {
        $env:GOOS = $target.GOOS
        $env:GOARCH = $target.GOARCH
        & $GoExe build -trimpath -ldflags "-s -w" -o (Join-Path $OutDir $target.Name) .
        if ($LASTEXITCODE -ne 0) { throw "go build failed for $($target.GOOS)/$($target.GOARCH)" }
        Write-Host "built $($target.Name)"
    }
}
finally {
    Pop-Location
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
}
