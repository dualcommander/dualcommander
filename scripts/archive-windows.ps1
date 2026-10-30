$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
Set-Location (Split-Path $PSScriptRoot -Parent)
$arch = go env GOARCH
if ($LASTEXITCODE -ne 0) { throw 'Cannot determine Go architecture' }
New-Item -ItemType Directory -Force dist | Out-Null
$runtimes = Get-Content packaging/windows/runtime.json -Raw | ConvertFrom-Json -AsHashtable
$runtime = $runtimes[$arch]
if (-not $runtime) { throw "Unsupported Windows architecture: $arch" }
$sevenZip = Get-Command 7z -ErrorAction SilentlyContinue
if (-not $sevenZip) { $sevenZip = Get-Command "$env:ProgramFiles/7-Zip/7z.exe" -ErrorAction Stop }
$work = Join-Path ([System.IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
New-Item -ItemType Directory "$work/stage/software" -Force | Out-Null
try {
    $url = "https://github.com/mmozeiko/build-mesa/releases/download/$($runtime.version)/mesa-llvmpipe-$($runtime.assetArchitecture)-$($runtime.version).7z"
    Invoke-WebRequest $url -OutFile "$work/mesa.7z"
    if ((Get-FileHash "$work/mesa.7z" -Algorithm SHA256).Hash -ne $runtime.sha256) {
        throw 'Mesa archive checksum mismatch'
    }
    & $sevenZip.Source x -y "-o$work/stage/software" "$work/mesa.7z" opengl32.dll
    if ($LASTEXITCODE -ne 0) { throw 'Mesa extraction failed' }
    $dll = [System.IO.File]::ReadAllBytes("$work/stage/software/opengl32.dll")
    $pe = [BitConverter]::ToInt32($dll, 0x3c)
    if ([BitConverter]::ToUInt32($dll, $pe) -ne 0x4550 -or
        [BitConverter]::ToUInt16($dll, $pe + 4) -ne @{ amd64 = 0x8664; arm64 = 0xaa64 }[$arch]) {
        throw "Mesa DLL architecture mismatch: $arch"
    }
    Copy-Item cmd/dualcommander/DualCommander.exe "$work/stage/DualCommander.exe"
    Copy-Item cmd/dualcommander/DualCommander.exe "$work/stage/software/DualCommander.exe"
    Copy-Item packaging/windows/mesa "$work/stage/mesa-licenses" -Recurse
    Copy-Item packaging/windows/runtime.json "$work/stage/mesa-licenses/runtime.json"
    Compress-Archive -Path "$work/stage/*" -DestinationPath "dist/dualcommander-windows-$arch.zip" -Force
} finally {
    Remove-Item $work -Recurse -Force
}