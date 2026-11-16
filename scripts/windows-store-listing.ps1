param(
    [ValidateSet('Validate', 'Export', 'Publish')]
    [string]$Action = 'Validate'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
Set-Location (Split-Path $PSScriptRoot -Parent)
$productId = '9NDGVQNPXVPT'
$path = 'packaging/windows/store-listing.json'
$fields = @('title', 'description', 'shortDescription', 'features', 'releaseNotes', 'keywords', 'licenseTerms', 'copyrightAndTrademarkInfo')

function Invoke-Store {
    $output = & msstore @args
    if ($LASTEXITCODE -ne 0) { throw "Microsoft Store CLI failed with exit code $LASTEXITCODE" }
    $output -join "`n"
}

if ($Action -ne 'Export') {
    $raw = Get-Content -Raw $path
    if ($raw.Contains('REPLACE_FROM_PARTNER_CENTER')) { throw "Replace placeholders in $path before publishing" }
    $text = $raw | ConvertFrom-Json
    if ($text -isnot [pscustomobject]) { throw 'Listing must be a JSON object' }
    foreach ($property in $text.PSObject.Properties) {
        if ($property.Name -cnotin $fields) { throw "Unsupported listing field: $($property.Name)" }
        $type = if ($property.Name -in @('features', 'keywords')) { [array] } else { [string] }
        if ($property.Value -isnot $type) { throw "Invalid type for $($property.Name): expected $type" }
        foreach ($value in @($property.Value)) {
            if ($value -isnot [string]) { throw "$($property.Name) must contain strings" }
        }
    }
    foreach ($name in @('title', 'description')) {
        if (-not $text.PSObject.Properties[$name] -or [string]::IsNullOrWhiteSpace($text.$name)) {
            throw "Missing listing $name"
        }
    }
    if ($Action -eq 'Validate') { return }
}

# MSIX updateMetadata replaces the whole submission. Always merge into fresh data.
$submission = Invoke-Store submission get $productId | ConvertFrom-Json
$listing = $submission.listings.'en-us'.baseListing
if (-not $listing) { throw 'Store submission has no English baseListing' }

if ($Action -eq 'Export') {
    $export = [ordered]@{}
    foreach ($name in $fields) {
        $property = $listing.PSObject.Properties[$name]
        if ($property -and $null -ne $property.Value) { $export[$name] = $property.Value }
    }
    New-Item -ItemType Directory -Force dist/windows-store | Out-Null
    $export | ConvertTo-Json -Depth 100 | Set-Content -Encoding utf8 dist/windows-store/store-listing.json
    return
}

foreach ($property in $text.PSObject.Properties) {
    $listing | Add-Member -NotePropertyName $property.Name -NotePropertyValue $property.Value -Force
}
New-Item -ItemType Directory -Force dist/windows-store | Out-Null
$payloadPath = 'dist/windows-store/submission.json'
$submission | ConvertTo-Json -Depth 100 | Set-Content -Encoding utf8 $payloadPath
Invoke-Store submission updateMetadata $productId $payloadPath | Out-Null
Invoke-Store submission publish $productId | Write-Host