$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
Set-Location (Split-Path $PSScriptRoot -Parent)
$arch = go env GOARCH
if ($LASTEXITCODE -ne 0) { throw 'Cannot determine Go architecture' }
New-Item -ItemType Directory -Force dist | Out-Null
Compress-Archive -Path cmd/dualcommander/DualCommander.exe -DestinationPath "dist/dualcommander-windows-$arch.zip" -Force