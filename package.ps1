# Packs two release archives: full (49 tools) and lite (12 tools).
param(
    [string]$Version = "1.1.0",
    [string]$OutDir = "$PSScriptRoot\dist"
)

$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

function New-Stage([string]$name) {
    $stage = Join-Path $env:TEMP "zero-ego-pack-$Version-$name"
    if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
    New-Item -ItemType Directory -Force -Path $stage | Out-Null
    return $stage
}

function Compress-Stage([string]$stage, [string]$zip) {
    if (Test-Path $zip) { Remove-Item $zip -Force }
    Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zip -CompressionLevel Optimal
    Remove-Item $stage -Recurse -Force
    Get-Item $zip | Select-Object Name, Length
}

# ---------- full ----------
$full = New-Stage "full"
foreach ($file in @("README.md", "AGENTS.md", "install.ps1", "install.sh", "build.ps1", "main.go", "main_test.go", "go.mod", "LICENSE")) {
    Copy-Item (Join-Path $PSScriptRoot $file) (Join-Path $full $file) -Force
}
Copy-Item (Join-Path $PSScriptRoot "python") (Join-Path $full "python") -Recurse -Force
Get-ChildItem (Join-Path $full "python") -Recurse -Directory -Filter "__pycache__" | Remove-Item -Recurse -Force
Get-ChildItem (Join-Path $PSScriptRoot "dist") -File |
    Where-Object { $_.Name -like "zero-ego-*" -and $_.Extension -ne ".zip" -and $_.Name -notlike "*lite*" } |
    Copy-Item -Destination $full -Force
Compress-Stage $full (Join-Path $OutDir "zero-ego-$Version.zip")

# ---------- lite ----------
$lite = New-Stage "lite"
foreach ($file in @("install.ps1", "install.sh", "LICENSE")) {
    Copy-Item (Join-Path $PSScriptRoot $file) (Join-Path $lite $file) -Force
}
if (Test-Path (Join-Path $PSScriptRoot "lite\README.md")) {
    Copy-Item (Join-Path $PSScriptRoot "lite\README.md") (Join-Path $lite "README.md") -Force
}
Copy-Item (Join-Path $PSScriptRoot "lite\main.go") (Join-Path $lite "main.go") -Force
Copy-Item (Join-Path $PSScriptRoot "lite\go.mod") (Join-Path $lite "go.mod") -Force
Get-ChildItem (Join-Path $PSScriptRoot "dist") -File |
    Where-Object { $_.Name -like "zero-ego-lite-*" -and $_.Extension -ne ".zip" } |
    Copy-Item -Destination $lite -Force
Compress-Stage $lite (Join-Path $OutDir "zero-ego-lite-$Version.zip")
