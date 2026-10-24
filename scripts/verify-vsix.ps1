[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Vsix,

    [string]$SourceExtension,

    [string]$RepositoryRoot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$errors = [System.Collections.Generic.List[string]]::new()
$zip = $null

function Get-StreamSha256([System.IO.Stream]$Stream) {
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        ([BitConverter]::ToString($sha.ComputeHash($Stream))).Replace('-', '').ToLowerInvariant()
    }
    finally {
        $sha.Dispose()
    }
}

function Get-FileSha256([string]$Path) {
    (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Get-BytesSha256([byte[]]$Bytes) {
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        ([BitConverter]::ToString($sha.ComputeHash($Bytes))).Replace('-', '').ToLowerInvariant()
    }
    finally {
        $sha.Dispose()
    }
}

function Read-ZipText([System.IO.Compression.ZipArchiveEntry]$Entry) {
    $stream = $Entry.Open()
    try {
        $reader = New-Object System.IO.StreamReader($stream, [System.Text.Encoding]::UTF8, $true)
        try { $reader.ReadToEnd() }
        finally { $reader.Dispose() }
    }
    finally { $stream.Dispose() }
}

try {
    $vsixPath = [System.IO.Path]::GetFullPath($Vsix)
    if (-not (Test-Path -LiteralPath $vsixPath -PathType Leaf)) {
        throw "VSIX not found: $vsixPath"
    }

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [System.IO.Compression.ZipFile]::OpenRead($vsixPath)
    $entryMap = @{}
    foreach ($entry in $zip.Entries) {
        $key = $entry.FullName.Replace('\', '/').ToLowerInvariant()
        if ($entryMap.ContainsKey($key)) { throw "Duplicate VSIX entry: $($entry.FullName)" }
        $entryMap[$key] = $entry
    }

    $manifestKey = 'extension/release-manifest.json'
    if (-not $entryMap.ContainsKey($manifestKey)) { throw 'VSIX is missing extension/release-manifest.json.' }
    $manifest = Read-ZipText $entryMap[$manifestKey] | ConvertFrom-Json
    if ($manifest.schema -ne 1) { throw "Unsupported release manifest schema: $($manifest.schema)" }
    if ($manifest.target -ne 'windows/amd64') { throw "Unexpected binary target in manifest: $($manifest.target)" }
    if (-not $entryMap.ContainsKey('extension.vsixmanifest')) { throw 'VSIX is missing extension.vsixmanifest.' }

    $expectedPaths = [System.Collections.Generic.List[string]]::new()
    foreach ($record in $manifest.files) {
        $relative = ([string]$record.path).Replace('\', '/').ToLowerInvariant()
        if ([string]::IsNullOrWhiteSpace($relative) -or $relative.StartsWith('/') -or $relative.Contains('..')) {
            throw "Unsafe manifest path: $relative"
        }
        $expectedPaths.Add("extension/$relative")
        $archiveKey = "extension/$relative"
        if (-not $entryMap.ContainsKey($archiveKey)) {
            $errors.Add("Missing packaged file: $archiveKey")
            continue
        }
        $stream = $entryMap[$archiveKey].Open()
        try { $actualHash = Get-StreamSha256 $stream }
        finally { $stream.Dispose() }
        if ($actualHash -ne ([string]$record.sha256).ToLowerInvariant()) {
            $errors.Add("Package hash mismatch: $archiveKey expected $($record.sha256), got $actualHash")
        }
    }
    $expectedPaths.Add($manifestKey)
    $actualExtensionPaths = @($zip.Entries | Where-Object { $_.FullName.Replace('\', '/').StartsWith('extension/', [System.StringComparison]::OrdinalIgnoreCase) } | ForEach-Object { $_.FullName.Replace('\', '/').ToLowerInvariant() } | Sort-Object -Unique)
    $expectedExtensionPaths = @($expectedPaths | Sort-Object -Unique)
    $pathDiff = Compare-Object -ReferenceObject $expectedExtensionPaths -DifferenceObject $actualExtensionPaths
    if ($pathDiff) {
        foreach ($diff in $pathDiff) { $errors.Add("Extension package inventory mismatch ($($diff.SideIndicator)): $($diff.InputObject)") }
    }

    $packageKey = 'extension/package.json'
    if (-not $entryMap.ContainsKey($packageKey)) { throw 'VSIX is missing extension/package.json.' }
    $package = Read-ZipText $entryMap[$packageKey] | ConvertFrom-Json
    if ($package.version -ne $manifest.version) {
        $errors.Add("Package version $($package.version) does not match manifest $($manifest.version).")
    }
    if ($package.name -ne $manifest.package) {
        $errors.Add("Package name $($package.name) does not match manifest $($manifest.package).")
    }

    if (-not [string]::IsNullOrWhiteSpace($SourceExtension)) {
        $sourceRoot = [System.IO.Path]::GetFullPath($SourceExtension)
        foreach ($record in $manifest.files) {
            $relative = ([string]$record.path).Replace('/', '\')
            if ($relative -ieq 'bin\intentlens.exe') { continue }
            $sourcePath = Join-Path $sourceRoot $relative
            if (-not (Test-Path -LiteralPath $sourcePath -PathType Leaf)) {
                $errors.Add("Source file missing: $relative")
                continue
            }
            $sourceHash = Get-FileSha256 $sourcePath
            if ($sourceHash -ne ([string]$record.sha256).ToLowerInvariant()) {
                $errors.Add("Source hash mismatch: $relative expected $($record.sha256), got $sourceHash")
            }
        }
    }

    $goSourcesVerified = 0
    if (-not [string]::IsNullOrWhiteSpace($RepositoryRoot)) {
        $repoRoot = [System.IO.Path]::GetFullPath($RepositoryRoot)
        $goRecords = @($manifest.source.goFiles | Sort-Object path)
        $canonical = (($goRecords | ForEach-Object { "$($_.path) $($_.sha256)" }) -join "`n") + "`n"
        $aggregateHash = Get-BytesSha256 ([System.Text.Encoding]::UTF8.GetBytes($canonical))
        if ($aggregateHash -ne ([string]$manifest.source.goSourceSha256).ToLowerInvariant()) {
            $errors.Add('Go source tree aggregate hash does not match the release manifest.')
        }
        foreach ($record in $goRecords) {
            $relative = ([string]$record.path).Replace('/', '\')
            $sourcePath = Join-Path $repoRoot $relative
            if (-not (Test-Path -LiteralPath $sourcePath -PathType Leaf)) {
                $errors.Add("Go source missing: $relative")
                continue
            }
            $sourceHash = Get-FileSha256 $sourcePath
            if ($sourceHash -ne ([string]$record.sha256).ToLowerInvariant()) {
                $errors.Add("Go source hash mismatch: $relative expected $($record.sha256), got $sourceHash")
            }
            else { $goSourcesVerified++ }
        }
        $lockPath = Join-Path $repoRoot 'scripts\release-tools\package-lock.json'
        if (Test-Path -LiteralPath $lockPath -PathType Leaf) {
            $lockHash = Get-FileSha256 $lockPath
            if ($lockHash -ne ([string]$manifest.packaging.vsceLockSha256).ToLowerInvariant()) {
                $errors.Add('The checked-in vsce package-lock.json differs from the lock used to build this package.')
            }
        }
    }

    $result = [ordered]@{
        integrity = $(if ($errors.Count -eq 0) { 'PASS' } else { 'FAIL' })
        version = $manifest.version
        target = $manifest.target
        filesVerified = $manifest.files.Count
        goSourcesVerified = $goSourcesVerified
        goVersion = $manifest.source.goVersion
        vsceVersion = $manifest.packaging.vsceVersion
        distributionReady = [bool]$manifest.distributionReady
        blockers = @($manifest.blockers)
        errors = @($errors.ToArray())
    }
    Write-Output ($result | ConvertTo-Json -Depth 20)
}
catch {
    $errors.Add($_.Exception.Message)
    $result = [ordered]@{ integrity = 'FAIL'; errors = @($errors.ToArray()) }
    Write-Output ($result | ConvertTo-Json -Depth 20)
}
finally {
    if ($zip) { $zip.Dispose() }
}

if ($errors.Count -gt 0) { exit 1 }
