param([string]$InputDirectory = 'dist')

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not $IsWindows) { throw 'MSIX packaging requires Windows and Windows SDK MakeAppx' }
Set-Location (Split-Path $PSScriptRoot -Parent)
foreach ($key in @('RELEASE_TAG', 'WINDOWS_STORE_IDENTITY_NAME', 'WINDOWS_STORE_PUBLISHER', 'WINDOWS_STORE_PUBLISHER_DISPLAY_NAME')) {
    if ([string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($key))) { throw "Missing $key" }
}
Get-Command python -ErrorAction Stop | Out-Null
$sdk = Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\bin" -Directory |
    Where-Object { Test-Path "$($_.FullName)\x64\makeappx.exe" } |
    Sort-Object { [version]$_.Name } -Descending | Select-Object -First 1
if (-not $sdk) { throw 'Windows SDK MakeAppx not found' }
$makeappx = "$($sdk.FullName)\x64\makeappx.exe"
foreach ($arch in @('amd64', 'arm64')) {
    if (-not (Test-Path "$InputDirectory/dualcommander-windows-$arch.zip" -PathType Leaf)) {
        throw "Missing $InputDirectory/dualcommander-windows-$arch.zip"
    }
}

# ponytail: package existing native builds only; build each architecture in CI.
$work = Join-Path ([System.IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
New-Item -ItemType Directory "$work/packages" -Force | Out-Null
try {
    Add-Type -AssemblyName System.Drawing
    foreach ($arch in @('amd64', 'arm64')) {
        $stage = "$work/$arch"
        New-Item -ItemType Directory "$stage/Assets" -Force | Out-Null
        Expand-Archive "$InputDirectory/dualcommander-windows-$arch.zip" -DestinationPath $stage
        if (-not (Test-Path "$stage/DualCommander.exe" -PathType Leaf)) { throw "Missing DualCommander.exe: $arch" }
        python scripts/windows-store.py "$stage/AppxManifest.xml" $arch
        if ($LASTEXITCODE -ne 0) { throw 'Store manifest generation failed' }
        $source = [System.Drawing.Image]::FromFile("$pwd/assets/dualcommander.png")
        try {
            foreach ($asset in @{ StoreLogo = 50; Logo = 150; SmallLogo = 44 }.GetEnumerator()) {
                $bitmap = [System.Drawing.Bitmap]::new($source, [int]$asset.Value, [int]$asset.Value)
                try { $bitmap.Save("$stage/Assets/$($asset.Key).png", [System.Drawing.Imaging.ImageFormat]::Png) }
                finally { $bitmap.Dispose() }
            }
        } finally { $source.Dispose() }
        & $makeappx pack /d $stage /p "$work/packages/dualcommander-$arch.msix" /o
        if ($LASTEXITCODE -ne 0) { throw "MSIX packaging failed: $arch" }
    }
    & $makeappx bundle /d "$work/packages" /p "$work/dualcommander.msixbundle" /o
    if ($LASTEXITCODE -ne 0) { throw 'MSIX bundling failed' }
    New-Item -ItemType Directory -Force dist/windows-store | Out-Null
    Copy-Item "$work/dualcommander.msixbundle" dist/windows-store/dualcommander.msixbundle -Force
    Write-Host 'Store upload bundle: dist/windows-store/dualcommander.msixbundle (unsigned; not sideload-ready)'
} finally {
    Remove-Item $work -Recurse -Force
}