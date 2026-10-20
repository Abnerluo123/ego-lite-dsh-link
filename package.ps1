# Packs the connector into a redistributable zip.
param(
    [string]$Version = "2.0.0",
    [string]$OutDir = "$PSScriptRoot\dist"
)

$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

$stage = Join-Path $env:TEMP "ego-browser-dsh-pack-$Version"
if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
New-Item -ItemType Directory -Force -Path $stage | Out-Null

$files = @(
    "README.md", "AGENTS.md", "install.ps1", "install.sh", "build.ps1",
    "main.go", "main_test.go", "go.mod"
)
foreach ($file in $files) {
    Copy-Item (Join-Path $PSScriptRoot $file) (Join-Path $stage $file) -Force
}
Copy-Item (Join-Path $PSScriptRoot "python") (Join-Path $stage "python") -Recurse -Force
Get-ChildItem (Join-Path $stage "python") -Recurse -Directory -Filter "__pycache__" |
    Remove-Item -Recurse -Force
Get-ChildItem (Join-Path $PSScriptRoot "dist") -File |
    Where-Object { $_.Name -like "ego-browser-dsh-*" -and $_.Extension -ne ".zip" } |
    Copy-Item -Destination $stage -Force

$zip = Join-Path $OutDir "ego-browser-dsh-link-$Version.zip"
if (Test-Path $zip) { Remove-Item $zip -Force }
Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zip -CompressionLevel Optimal
Remove-Item $stage -Recurse -Force

Get-Item $zip | Select-Object FullName, Length
