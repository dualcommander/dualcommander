$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
Set-Location (Split-Path $PSScriptRoot -Parent)
if (-not $env:EXPECTED_SIGNER) { throw 'Missing WINDOWS_SIGNER_SUBJECT' }
$exe = 'signed-windows/DualCommander.exe'
$signature = Get-AuthenticodeSignature $exe
if ($signature.Status -ne 'Valid') { throw "Invalid signature: $($signature.Status)" }
if ($signature.SignerCertificate.Subject -ne $env:EXPECTED_SIGNER) { throw 'Unexpected signing identity' }
if (-not $signature.TimeStamperCertificate) { throw 'Missing Authenticode timestamp' }
Copy-Item $exe cmd/dualcommander/DualCommander.exe -Force