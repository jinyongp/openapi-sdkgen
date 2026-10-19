[CmdletBinding()]
param(
    [string]$Version = '@VERSION@',
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Programs/openapi-sdkgen/bin')
)
$ErrorActionPreference = 'Stop'
$Version = $Version -creplace '^v', ''
if ($Version -cnotmatch '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$') {
    throw 'Invalid version; use a released installer or -Version X.Y.Z'
}
if ($env:OS -ne 'Windows_NT') { throw 'Use install.sh on macOS and Linux' }
# PROCESSOR_ARCHITEW6432 reports the native architecture from an emulated process.
$nativeArch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
$arch = switch ($nativeArch) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { throw 'Supported CPU architectures: amd64 and arm64' }
}
if ([string]::IsNullOrWhiteSpace($InstallDir)) { throw 'InstallDir must not be empty' }
$InstallDir = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($InstallDir)
$asset = "openapi-sdkgen_${Version}_windows_${arch}.zip"
$base = "https://github.com/jinyongp/openapi-sdkgen/releases/download/v$Version"
$work = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
$staged = $null
$backup = $null
$previousProtocol = [Net.ServicePointManager]::SecurityProtocol
try {
    [Net.ServicePointManager]::SecurityProtocol = $previousProtocol -bor [Net.SecurityProtocolType]::Tls12
    New-Item -ItemType Directory -Path $work | Out-Null
    $checksums = Join-Path $work 'checksums.txt'
    $archivePath = Join-Path $work $asset
    Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile $checksums -TimeoutSec 300
    Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile $archivePath -TimeoutSec 300
    $entries = @(Get-Content -LiteralPath $checksums | Where-Object { $_ -cmatch ('^([0-9a-fA-F]{64})\s+\*?' + [regex]::Escape($asset) + '$') })
    if ($entries.Count -ne 1) { throw 'Expected exactly one SHA-256 checksum for the release archive' }
    $expected = ($entries[0] -split '\s+')[0]
    if ((Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash -ine $expected) {
        throw 'Checksum mismatch; installation was not changed'
    }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $archive = [IO.Compression.ZipFile]::OpenRead($archivePath)
    try {
        $binaries = @($archive.Entries | Where-Object { $_.FullName -ceq 'openapi-sdkgen.exe' })
        if ($binaries.Count -ne 1) { throw 'Archive must contain exactly one executable' }
        $binary = Join-Path $work 'openapi-sdkgen.exe'
        [IO.Compression.ZipFileExtensions]::ExtractToFile($binaries[0], $binary, $false)
    } finally { $archive.Dispose() }
    [IO.Directory]::CreateDirectory($InstallDir) | Out-Null
    $destination = Join-Path $InstallDir 'openapi-sdkgen.exe'
    if (Test-Path -LiteralPath $destination) {
        $existing = Get-Item -LiteralPath $destination -Force
        if ($existing.PSIsContainer -or ($existing.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            throw "Refusing to replace a directory or link: $destination"
        }
    }
    $staged = Join-Path $InstallDir ('.openapi-sdkgen.' + [Guid]::NewGuid().ToString())
    [IO.File]::Copy($binary, $staged, $false)
    if ([IO.File]::Exists($destination)) {
        $backup = "$staged.previous"
        [IO.File]::Replace($staged, $destination, $backup)
    }
    else { [IO.File]::Move($staged, $destination) }
    $staged = $null
    Write-Output "Installed openapi-sdkgen $Version to $destination"
    if (($env:PATH -split ';') -inotcontains $InstallDir) { Write-Output "Add this directory to PATH: $InstallDir" }
} finally {
    [Net.ServicePointManager]::SecurityProtocol = $previousProtocol
    if ($staged -and (Test-Path -LiteralPath $staged)) { Remove-Item -LiteralPath $staged -Force }
    if ($backup -and (Test-Path -LiteralPath $backup)) { Remove-Item -LiteralPath $backup -Force }
    if (Test-Path -LiteralPath $work) { Remove-Item -LiteralPath $work -Recurse -Force }
}
