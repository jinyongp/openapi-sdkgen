$ErrorActionPreference = 'Stop'
$installer = Join-Path $PSScriptRoot 'install.ps1'
$root = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $root | Out-Null
$global:SdkgenInstallFixture = @{ Root = $root; Mode = ''; Asset = '' }
$saved = @{}
$previousCasedPath = [Environment]::GetEnvironmentVariable('Path')
foreach ($key in @('OS', 'PROCESSOR_ARCHITECTURE', 'PROCESSOR_ARCHITEW6432', 'LOCALAPPDATA', 'PATH')) { $saved[$key] = [Environment]::GetEnvironmentVariable($key) }
function Assert-True($Condition, $Message) { if (-not $Condition) { throw $Message } }
function Invoke-WebRequest([switch]$UseBasicParsing, $Uri, $OutFile, $TimeoutSec) {
    if ($global:SdkgenInstallFixture.Mode -eq 'download') { throw 'Fixture download failure' }
    $base = 'https://github.com/jinyongp/openapi-sdkgen/releases/download/v1.2.3'
    if ($Uri -ceq "$base/checksums.txt") { [IO.File]::Copy((Join-Path $global:SdkgenInstallFixture.Root 'checksums.txt'), $OutFile) }
    elseif ($Uri -ceq "$base/$($global:SdkgenInstallFixture.Asset)") { [IO.File]::Copy((Join-Path $global:SdkgenInstallFixture.Root 'archive.zip'), $OutFile) }
    else { throw "Unexpected URL: $Uri" }
}
function Get-Command($Name, $ErrorAction) { return $global:SdkgenInstallFixture.CurrentCommand }
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
            $output = (& $installer -Version v1.2.3) -join "`n"
            $previous = -1
            foreach ($message in @('Installing openapi-sdkgen 1.2.3', "Destination: $destination", 'Downloading release checksums', "Downloading $($global:SdkgenInstallFixture.Asset)", 'Verifying SHA-256', 'Extracting executable', 'Installing executable', 'Installed openapi-sdkgen 1.2.3', 'Run now:')) {
                $position = $output.IndexOf($message)
                Assert-True ($position -gt $previous) "Missing or out-of-order guidance: $message"
                $previous = $position
            }
            Assert-True ((Get-Content -LiteralPath $destination -Raw) -ceq 'fixture executable') 'Install/reinstall content mismatch'
            Assert-True (@(Get-ChildItem -LiteralPath (Split-Path $destination) -Force).Count -eq 1) 'Reinstall left staged files'
        }
    }
    $env:PROCESSOR_ARCHITECTURE = 'x86'
    $env:PROCESSOR_ARCHITEW6432 = 'ARM64'
    $stamped = Join-Path $root 'install.ps1'
    [IO.File]::WriteAllText($stamped, ([IO.File]::ReadAllText($installer).Replace('@VERSION@', '1.2.3')))
    $ErrorActionPreference = 'Continue'
    $previousProgress = $ProgressPreference
    $protocol = [Net.ServicePointManager]::SecurityProtocol
    [IO.File]::ReadAllText($stamped) | Invoke-Expression | Out-Null
    Assert-True ($ErrorActionPreference -eq 'Continue') 'Piped installer changed caller error preference'
    Assert-True ($ProgressPreference -eq $previousProgress) 'Piped installer changed caller progress preference'
    Assert-True ([Net.ServicePointManager]::SecurityProtocol -eq $protocol) 'Piped installer changed TLS configuration'
    $failed = $false
    try { [IO.File]::ReadAllText($installer).Replace('@VERSION@', 'invalid') | Invoke-Expression | Out-Null } catch { $failed = $true }
    Assert-True $failed 'Piped installer accepted invalid version'
    Assert-True ($ErrorActionPreference -eq 'Continue') 'Failed piped installer changed caller error preference'
    $ErrorActionPreference = 'Stop'
    $defaultDestination = Join-Path $env:LOCALAPPDATA 'Programs/openapi-sdkgen/bin/openapi-sdkgen.exe'
    Assert-True ((Get-Content -LiteralPath $defaultDestination -Raw) -ceq 'fixture executable') 'Piped installer default failed'
    $custom = Join-Path $root "custom ' `$literal path"
    $global:SdkgenInstallFixture.CurrentCommand = [pscustomobject]@{ CommandType = 'Application'; Source = 'old executable' }
    $output = (& $stamped -InstallDir $custom) -join "`n"
    $destination = Join-Path $custom 'openapi-sdkgen.exe'
    Assert-True ((Get-Content -LiteralPath $destination -Raw) -ceq 'fixture executable') 'Stamped default/native architecture failed'
    Assert-True ($output.Contains('Another executable currently takes precedence: old executable')) 'Missing old-command warning'
    $pathLine = ($output -split "`n" | Where-Object { $_.TrimStart().StartsWith('$env:Path = ') }).Trim()
    Invoke-Expression $pathLine
    Assert-True ($env:Path.StartsWith("$custom;")) 'Printed PATH command did not preserve literal directory'
    $runLine = ($output -split "`n" | Where-Object { $_.TrimStart().StartsWith('& ') }).Trim()
    $tokens = $null; $parseErrors = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseInput($runLine, [ref]$tokens, [ref]$parseErrors)
    Assert-True ($parseErrors.Count -eq 0) 'Printed executable command is invalid PowerShell'
    $literal = $ast.Find({ param($node) $node -is [System.Management.Automation.Language.StringConstantExpressionAst] -and $node.Value -ceq $destination }, $true)
    Assert-True ($null -ne $literal) 'Printed executable command changed the literal path'
    $global:SdkgenInstallFixture.CurrentCommand = [pscustomobject]@{ CommandType = 'Application'; Source = $destination }
    $output = (& $stamped -InstallDir $custom) -join "`n"
    Assert-True ($output.Contains("Verify installation:`n  openapi-sdkgen --version")) 'Missing ready-to-run confirmation'
    Assert-True (-not $output.Contains('$env:Path = ')) 'Unneeded PATH change when command already resolves correctly'
    $global:SdkgenInstallFixture.CurrentCommand = $null
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
        $global:SdkgenInstallFixture.Output = [Collections.Generic.List[string]]::new()
        try { & $installer -Version 1.2.3 -InstallDir $custom | ForEach-Object { $global:SdkgenInstallFixture.Output.Add($_) } } catch {
            $failed = $true
            Assert-True ($_.Exception.Message.Contains('openapi-sdkgen:')) "Missing installer error context for $mode"
            if ($mode -eq 'download') { Assert-True ($_.Exception.Message.Contains('Check your connection')) 'Missing download recovery hint' }
        }
        Assert-True (-not (($global:SdkgenInstallFixture.Output -join "`n").Contains('Installed openapi-sdkgen'))) 'Failed installer printed success'
        Assert-True $failed "Expected $mode failure"
        Assert-True ((Get-Content -LiteralPath $destination -Raw) -ceq 'old installation') "$mode replaced an existing installation"
        Assert-True (@(Get-ChildItem -LiteralPath $custom -Force).Count -eq 1) "$mode left staged files"
    }
    $global:SdkgenInstallFixture.Mode = ''
    [IO.File]::WriteAllBytes((Join-Path $root 'archive.zip'), $archiveBytes)
    [IO.File]::WriteAllText((Join-Path $root 'checksums.txt'), "$hash  $($global:SdkgenInstallFixture.Asset)`n")
    $blocked = Join-Path $root 'blocked'
    [IO.File]::WriteAllText($blocked, 'retain')
    $failed = $false
    try { & $installer -Version 1.2.3 -InstallDir $blocked | Out-Null } catch {
        $failed = $true
        Assert-True ($_.Exception.Message.Contains('Installing executable')) 'Missing install failure stage'
        Assert-True ($_.Exception.Message.Contains('-InstallDir')) 'Missing writable-directory recovery hint'
    }
    Assert-True $failed 'Accepted blocked directory'
    Assert-True ((Get-Content -LiteralPath $blocked -Raw) -ceq 'retain') 'Changed blocked destination'
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
    [Environment]::SetEnvironmentVariable('Path', $previousCasedPath)
    Remove-Variable -Name SdkgenInstallFixture -Scope Global
    Remove-Item -LiteralPath $root -Recurse -Force
}
