[CmdletBinding()]
param([Parameter(Mandatory = $true)][string]$Vsix)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$verifier = Join-Path $PSScriptRoot 'verify-vsix.ps1'
$hostExe = (Get-Process -Id $PID).Path
$temporary = Join-Path ([IO.Path]::GetTempPath()) ('intentlens-integrity-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temporary | Out-Null

function Verify([string]$Path, [bool]$ShouldPass) {
    $raw = (& $hostExe -NoProfile -File $verifier -Vsix $Path -SourceExtension (Join-Path $repo 'extension') -RepositoryRoot $repo | Out-String)
    $code = $LASTEXITCODE
    $result = $raw | ConvertFrom-Json
    if ($ShouldPass -and ($code -ne 0 -or $result.integrity -ne 'PASS')) { throw "Valid candidate rejected: $raw" }
    if (-not $ShouldPass -and ($code -eq 0 -or $result.integrity -ne 'FAIL')) { throw 'Tampered candidate was accepted.' }
}

try {
    Verify ([IO.Path]::GetFullPath($Vsix)) $true
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    foreach ($kind in @('source-tamper', 'missing-manifest', 'publisher-mismatch', 'id-mismatch', 'version-mismatch')) {
        $copy = Join-Path $temporary "$kind.vsix"
        Copy-Item -LiteralPath $Vsix -Destination $copy
        $zip = [IO.Compression.ZipFile]::Open($copy, [IO.Compression.ZipArchiveMode]::Update)
        try {
            if ($kind -eq 'source-tamper') {
                $entry = $zip.GetEntry('extension/src/core.js')
                if ($null -eq $entry) { throw 'Candidate missing core.js.' }
                $entry.Delete()
                $entry = $zip.CreateEntry('extension/src/core.js')
                $writer = [IO.StreamWriter]::new($entry.Open())
                try { $writer.Write('tampered fixture') } finally { $writer.Dispose() }
            } elseif ($kind -eq 'missing-manifest') {
                $entry = $zip.GetEntry('extension/release-manifest.json')
                if ($null -eq $entry) { throw 'Candidate missing release manifest.' }
                $entry.Delete()
            } else {
                $entry = $zip.GetEntry('extension.vsixmanifest')
                if ($null -eq $entry) { throw 'Candidate missing VSIX manifest.' }
                $reader = [IO.StreamReader]::new($entry.Open())
                try { $xml = [xml]$reader.ReadToEnd() } finally { $reader.Dispose() }
                $identity = $xml.SelectSingleNode("//*[local-name()='Metadata']/*[local-name()='Identity']")
                if ($null -eq $identity) { throw 'Candidate VSIX manifest has no Identity element.' }
                $attribute = switch ($kind) {
                    'publisher-mismatch' { 'Publisher' }
                    'id-mismatch' { 'Id' }
                    'version-mismatch' { 'Version' }
                }
                $identity.SetAttribute($attribute, "tampered-$attribute")
                $entry.Delete()
                $entry = $zip.CreateEntry('extension.vsixmanifest')
                $writer = [IO.StreamWriter]::new($entry.Open(), [Text.UTF8Encoding]::new($false))
                try { $xml.Save($writer) } finally { $writer.Dispose() }
            }
        } finally { $zip.Dispose() }
        Verify $copy $false
    }
    Write-Output 'PASS: candidate integrity, source/manifest tamper rejection, and VSIX identity mismatch rejection.'
} finally {
    Remove-Item -LiteralPath $temporary -Recurse -Force
}
exit 0
