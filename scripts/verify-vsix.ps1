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
    if ($manifest.schema -ne 2) { throw "Unsupported release manifest schema: $($manifest.schema)" }
    if ($manifest.channel -ne 'manual-vsix') { throw "Unexpected release channel: $($manifest.channel)" }
    if ([string]::IsNullOrWhiteSpace($manifest.publisher) -or $manifest.license -ne 'MIT') {
        throw 'Release manifest publisher or license is missing or unsupported.'
    }
    if ($null -eq $manifest.marketplacePublisherVerified -or $null -eq $manifest.marketplaceReady -or $null -eq $manifest.blockers) {
        throw 'Release manifest is missing Marketplace readiness fields.'
    }
    if ($manifest.marketplacePublisherVerified -ne $false -or $manifest.marketplaceReady -ne $false -or
        @($manifest.blockers) -notcontains 'publisher_ownership_unverified') {
        throw 'Manual VSIX manifests cannot attest Marketplace publisher ownership or readiness.'
    }
    if ($manifest.target -ne 'windows/amd64') { throw "Unexpected binary target in manifest: $($manifest.target)" }
    if (-not $entryMap.ContainsKey('extension.vsixmanifest')) { throw 'VSIX is missing extension.vsixmanifest.' }

    $vsixManifestText = Read-ZipText $entryMap['extension.vsixmanifest']
    $xmlSettings = [System.Xml.XmlReaderSettings]::new()
    $xmlSettings.DtdProcessing = [System.Xml.DtdProcessing]::Prohibit
    $xmlSettings.XmlResolver = $null
    $xmlStringReader = [System.IO.StringReader]::new($vsixManifestText)
    $xmlReader = $null
    try {
        $xmlReader = [System.Xml.XmlReader]::Create($xmlStringReader, $xmlSettings)
        $vsixXml = [System.Xml.XmlDocument]::new()
        $vsixXml.XmlResolver = $null
        $vsixXml.Load($xmlReader)
    }
    finally {
        if ($xmlReader) { $xmlReader.Dispose() }
        $xmlStringReader.Dispose()
    }
    $identity = $vsixXml.SelectSingleNode("/*[local-name()='PackageManifest']/*[local-name()='Metadata']/*[local-name()='Identity']")
    if (-not $identity) { throw 'VSIX manifest is missing Metadata/Identity.' }

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
    if ($package.publisher -ne $manifest.publisher) {
        $errors.Add('Package publisher does not match the release manifest.')
    }
    if ($package.license -ne $manifest.license) {
        $errors.Add('Package license does not match the release manifest.')
    }
    if ($identity.GetAttribute('Publisher') -cne $package.publisher) {
        $errors.Add('VSIX manifest publisher does not match package.json.')
    }
    if ($identity.GetAttribute('Id') -cne $package.name) {
        $errors.Add('VSIX manifest extension ID does not match package.json name.')
    }
    if ($identity.GetAttribute('Version') -cne $package.version) {
        $errors.Add('VSIX manifest version does not match package.json.')
    }
    if ($package.license -eq 'MIT') {
        if (-not $entryMap.ContainsKey('extension/license.txt')) {
            $errors.Add('MIT package is missing extension/LICENSE.txt from the VSIX.')
        } elseif ((Read-ZipText $entryMap['extension/license.txt']) -notmatch '(?m)^MIT License\s*$') {
            $errors.Add('VSIX package declares MIT but does not contain the MIT License text.')
        }
    }

    if (-not [string]::IsNullOrWhiteSpace($SourceExtension)) {
        $sourceRoot = [System.IO.Path]::GetFullPath($SourceExtension)
        foreach ($record in $manifest.files) {
            $relative = ([string]$record.path).Replace('/', '\')
            if ($relative -ieq 'bin\intentlens.exe') { continue }
            $sourcePath = Join-Path $sourceRoot $relative
            if ($relative -ieq 'license.txt' -and -not (Test-Path -LiteralPath $sourcePath -PathType Leaf)) {
                $sourcePath = Join-Path $sourceRoot 'LICENSE'
            }
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
        channel = $manifest.channel
        publisher = $manifest.publisher
        license = $manifest.license
        marketplaceReady = [bool]$manifest.marketplaceReady
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
