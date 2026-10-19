$ErrorActionPreference = 'Stop'
$installer = Join-Path $PSScriptRoot 'install.ps1'
$root = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $root | Out-Null
$global:SdkgenInstallFixture = @{ Root = $root; Mode = ''; Asset = '' }
$saved = @{}
foreach ($key in @('OS', 'PROCESSOR_ARCHITECTURE', 'PROCESSOR_ARCHITEW6432', 'LOCALAPPDATA')) { $saved[$key] = [Environment]::GetEnvironmentVariable($key) }
function Assert-True($Condition, $Message) { if (-not $Condition) { throw $Message } }
function Invoke-WebRequest([switch]$UseBasicParsing, $Uri, $OutFile, $TimeoutSec) {
    if ($global:SdkgenInstallFixture.Mode -eq 'download') { throw 'Fixture download failure' }
    $base = 'https://github.com/jinyongp/openapi-sdkgen/releases/download/v1.2.3'
    if ($Uri -ceq "$base/checksums.txt") { [IO.File]::Copy((Join-Path $global:SdkgenInstallFixture.Root 'checksums.txt'), $OutFile) }
    elseif ($Uri -ceq "$base/$($global:SdkgenInstallFixture.Asset)") { [IO.File]::Copy((Join-Path $global:SdkgenInstallFixture.Root 'archive.zip'), $OutFile) }
    else { throw "Unexpected URL: $Uri" }
}
try {
    $env:OS = 'Windows_NT'
    $env:LOCALAPPDATA = Join-Path $root 'local app data'
    $env:PROCESSOR_ARCHITEW6432 = ''
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $files = Join-Path $root 'files'
    New-Item -ItemType Directory -Path $files | Out-Null
    [IO.File]::WriteAllText((Join-Path $files 'openapi-sdkgen.exe'), 'fixture executable')
    [IO.Compression.ZipFile]::CreateFromDirectory($files, (Join-Path $root 'archive.zip'))
    $archiveBytes = [IO.File]::ReadAllBytes((Join-Path $root 'archive.zip'))
    $hash = (Get-FileHash -LiteralPath (Join-Path $root 'archive.zip') -Algorithm SHA256).Hash
    foreach ($arch in @('AMD64', 'ARM64')) {
        $env:PROCESSOR_ARCHITECTURE = $arch
        $global:SdkgenInstallFixture.Asset = "openapi-sdkgen_1.2.3_windows_$($arch.ToLowerInvariant()).zip"
        [IO.File]::WriteAllText((Join-Path $root 'checksums.txt'), "$hash  $($global:SdkgenInstallFixture.Asset)`n")
        $destination = Join-Path $env:LOCALAPPDATA 'Programs/openapi-sdkgen/bin/openapi-sdkgen.exe'
        1..2 | ForEach-Object {
            & $installer -Version v1.2.3 | Out-Null
            Assert-True ((Get-Content -LiteralPath $destination -Raw) -ceq 'fixture executable') 'Install/reinstall content mismatch'
            Assert-True (@(Get-ChildItem -LiteralPath (Split-Path $destination) -Force).Count -eq 1) 'Reinstall left staged files'
        }
    }
    $env:PROCESSOR_ARCHITECTURE = 'x86'
    $env:PROCESSOR_ARCHITEW6432 = 'ARM64'
    $stamped = Join-Path $root 'install.ps1'
    [IO.File]::WriteAllText($stamped, ([IO.File]::ReadAllText($installer).Replace('@VERSION@', '1.2.3')))
    $custom = Join-Path $root 'custom path'
    & $stamped -InstallDir $custom | Out-Null
    $destination = Join-Path $custom 'openapi-sdkgen.exe'
    Assert-True ((Get-Content -LiteralPath $destination -Raw) -ceq 'fixture executable') 'Stamped default/native architecture failed'
    foreach ($mode in @('download', 'tamper', 'missing', 'duplicate', 'bad-archive')) {
        $global:SdkgenInstallFixture.Mode = $mode
        [IO.File]::WriteAllBytes((Join-Path $root 'archive.zip'), $archiveBytes)
        [IO.File]::WriteAllText($destination, 'old installation')
        $checksum = "$hash  $($global:SdkgenInstallFixture.Asset)`n"
        if ($mode -eq 'tamper') { $checksum = ('0' * 64) + "  $($global:SdkgenInstallFixture.Asset)`n" }
        if ($mode -eq 'missing') { $checksum = "$hash  different.zip`n" }
        if ($mode -eq 'duplicate') { $checksum += $checksum }
        if ($mode -eq 'bad-archive') {
            [IO.File]::WriteAllText((Join-Path $root 'archive.zip'), 'not an archive')
            $badHash = (Get-FileHash -LiteralPath (Join-Path $root 'archive.zip') -Algorithm SHA256).Hash
            $checksum = "$badHash  $($global:SdkgenInstallFixture.Asset)`n"
        }
        [IO.File]::WriteAllText((Join-Path $root 'checksums.txt'), $checksum)
        $failed = $false
        try { & $installer -Version 1.2.3 -InstallDir $custom | Out-Null } catch { $failed = $true }
        Assert-True $failed "Expected $mode failure"
        Assert-True ((Get-Content -LiteralPath $destination -Raw) -ceq 'old installation') "$mode replaced an existing installation"
        Assert-True (@(Get-ChildItem -LiteralPath $custom -Force).Count -eq 1) "$mode left staged files"
    }
    $global:SdkgenInstallFixture.Mode = ''
    $failed = $false
    try { & $installer -Version '../1.2.3' -InstallDir $custom | Out-Null } catch { $failed = $true }
    Assert-True $failed 'Accepted invalid version'
    $env:PROCESSOR_ARCHITEW6432 = ''
    $failed = $false
    try { & $installer -Version 1.2.3 -InstallDir $custom | Out-Null } catch { $failed = $true }
    Assert-True $failed 'Accepted unsupported architecture'
    $env:PROCESSOR_ARCHITECTURE = 'AMD64'
    $failed = $false
    try { & $installer -Version 1.2.3 -InstallDir '' | Out-Null } catch { $failed = $true }
    Assert-True $failed 'Accepted empty install directory'
    Write-Output 'ok PowerShell installer: architectures, defaults, pinned version, reinstall and failure preservation'
} finally {
    foreach ($key in $saved.Keys) { [Environment]::SetEnvironmentVariable($key, $saved[$key]) }
    Remove-Variable -Name SdkgenInstallFixture -Scope Global
    Remove-Item -LiteralPath $root -Recurse -Force
}
