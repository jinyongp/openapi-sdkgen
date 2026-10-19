[CmdletBinding()]
param(
    [string]$Version = '@VERSION@',
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Programs/openapi-sdkgen/bin')
)
# Keep preferences local when invoked through irm | iex.
& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue'
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
    $destination = Join-Path $InstallDir 'openapi-sdkgen.exe'
    $asset = "openapi-sdkgen_${Version}_windows_${arch}.zip"
    $base = "https://github.com/jinyongp/openapi-sdkgen/releases/download/v$Version"
    $work = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
    $staged = $null
    $backup = $null
    $phase = 'Preparing installation'
    $previousProtocol = [Net.ServicePointManager]::SecurityProtocol
    Write-Output "Installing openapi-sdkgen $Version (windows/$arch)"
    Write-Output "Destination: $destination"
    try {
        [Net.ServicePointManager]::SecurityProtocol = $previousProtocol -bor [Net.SecurityProtocolType]::Tls12
        New-Item -ItemType Directory -Path $work | Out-Null
        $checksums = Join-Path $work 'checksums.txt'
        $archivePath = Join-Path $work $asset
        $phase = 'Downloading release checksums'
        Write-Output "$phase..."
        Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile $checksums -TimeoutSec 300
        $phase = "Downloading $asset"
        Write-Output "$phase..."
        Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile $archivePath -TimeoutSec 300
        $phase = 'Verifying SHA-256'
        Write-Output "$phase..."
        $entries = @(Get-Content -LiteralPath $checksums | Where-Object { $_ -cmatch ('^([0-9a-fA-F]{64})\s+\*?' + [regex]::Escape($asset) + '$') })
        if ($entries.Count -ne 1) { throw 'Expected exactly one SHA-256 checksum for the release archive' }
        $expected = ($entries[0] -split '\s+')[0]
        if ((Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash -ine $expected) {
            throw 'Checksum mismatch; installation was not changed'
        }
        $phase = 'Extracting executable'
        Write-Output "$phase..."
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        $archive = [IO.Compression.ZipFile]::OpenRead($archivePath)
        try {
            $binaries = @($archive.Entries | Where-Object { $_.FullName -ceq 'openapi-sdkgen.exe' })
            if ($binaries.Count -ne 1) { throw 'Archive must contain exactly one executable' }
            $binary = Join-Path $work 'openapi-sdkgen.exe'
            [IO.Compression.ZipFileExtensions]::ExtractToFile($binaries[0], $binary, $false)
        } finally { $archive.Dispose() }
        $phase = "Installing executable to $InstallDir"
        Write-Output "$phase..."
        [IO.Directory]::CreateDirectory($InstallDir) | Out-Null
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
    } catch {
        $hint = if ($phase -like 'Downloading*') { ' Check your connection and the release version.' }
            elseif ($phase -like 'Installing*') { ' Check permissions or choose a writable directory with -InstallDir.' }
            else { '' }
        throw "openapi-sdkgen: $phase failed. $($_.Exception.Message)$hint"
    } finally {
        [Net.ServicePointManager]::SecurityProtocol = $previousProtocol
        if ($staged -and (Test-Path -LiteralPath $staged)) { Remove-Item -LiteralPath $staged -Force }
        if ($backup -and (Test-Path -LiteralPath $backup)) { Remove-Item -LiteralPath $backup -Force }
        if (Test-Path -LiteralPath $work) { Remove-Item -LiteralPath $work -Recurse -Force }
    }
    $current = Get-Command openapi-sdkgen -ErrorAction SilentlyContinue
    if ($current -and $current.CommandType -eq 'Application' -and $current.Source -ieq $destination) {
        Write-Output "Verify installation:`n  openapi-sdkgen --version"
    } else {
        $quotedExecutable = $destination.Replace("'", "''")
        $quotedDir = $InstallDir.Replace("'", "''")
        Write-Output "Run now:`n  & '$quotedExecutable' --version"
        if ($current -and $current.CommandType -eq 'Application') {
            Write-Output "Another executable currently takes precedence: $($current.Source)"
        } elseif ($current) {
            Write-Output 'A PowerShell alias or function named openapi-sdkgen takes precedence. Use the full path above.'
        }
        Write-Output "Put this directory first in PATH for the current PowerShell session:`n  `$env:Path = '$quotedDir;' + `$env:Path"
        Write-Output 'For future terminals, add this directory to your user PATH in Environment Variables.'
    }
}
