$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
Set-Location (Split-Path $PSScriptRoot -Parent)
foreach ($key in @('WINDOWS_STORE_TENANT_ID', 'WINDOWS_STORE_SELLER_ID', 'WINDOWS_STORE_CLIENT_ID', 'WINDOWS_STORE_CLIENT_SECRET', 'WINDOWS_STORE_PRODUCT_ID')) {
    if (-not [Environment]::GetEnvironmentVariable($key)) { throw "Missing $key" }
}
msstore reconfigure --tenantId $env:WINDOWS_STORE_TENANT_ID --sellerId $env:WINDOWS_STORE_SELLER_ID --clientId $env:WINDOWS_STORE_CLIENT_ID --clientSecret $env:WINDOWS_STORE_CLIENT_SECRET
if ($LASTEXITCODE -ne 0) { throw 'Store authentication failed' }
msstore publish dist/windows-store/dualcommander.msixbundle --appId $env:WINDOWS_STORE_PRODUCT_ID
if ($LASTEXITCODE -ne 0) { throw 'Store submission failed' }