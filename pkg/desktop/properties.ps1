$ErrorActionPreference = 'Stop'
$path = [Environment]::GetEnvironmentVariable('DUALCOMMANDER_ITEM')
$shell = New-Object -ComObject Shell.Application
$folder = $shell.Namespace([IO.Path]::GetDirectoryName($path))
$item = $folder.ParseName([IO.Path]::GetFileName($path))
if ($null -eq $item) { throw 'Item no longer exists' }
$item.InvokeVerb('properties')