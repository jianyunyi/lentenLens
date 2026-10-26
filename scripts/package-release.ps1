[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$OutputDirectory,

    [ValidateSet('win32-x64')]
    [string]$Target = 'win32-x64',

    [switch]$Offline
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Get-Sha256([string]$Path) {
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

function Get-RelativePath([string]$Root, [string]$Path) {
    $Path.Substring($Root.TrimEnd('\').Length).TrimStart('\', '/').Replace('\', '/')
}

$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$extensionRoot = Join-Path $repoRoot 'extension'
$configPath = Join-Path $PSScriptRoot 'release-config.json'
$toolsRoot = Join-Path $PSScriptRoot 'release-tools'
$toolsPackagePath = Join-Path $toolsRoot 'package.json'
$toolsLockPath = Join-Path $toolsRoot 'package-lock.json'
$config = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
$toolsPackage = Get-Content -LiteralPath $toolsPackagePath -Raw | ConvertFrom-Json
$lockInfoJson = (& node -e "const l=require(process.argv[1]);process.stdout.write(JSON.stringify({version:l.lockfileVersion,vsce:l.packages['node_modules/@vscode/vsce']?.version}));" $toolsLockPath 2>&1 | Out-String).Trim()
if ($LASTEXITCODE -ne 0) { throw "Unable to read the pinned npm lock file: $lockInfoJson" }
$lockInfo = $lockInfoJson | ConvertFrom-Json
$extensionPackagePath = Join-Path $extensionRoot 'package.json'
$extensionPackage = Get-Content -LiteralPath $extensionPackagePath -Raw | ConvertFrom-Json

if ($Target -ne $config.target.vscode) {
    throw "Target mismatch: configured $($config.target.vscode), requested $Target."
}
if ($toolsPackage.devDependencies.'@vscode/vsce' -ne $config.vsceVersion) {
    throw "release-tools/package.json must pin @vscode/vsce exactly to $($config.vsceVersion)."
}
$lockedVsce = $lockInfo.vsce
if ($lockedVsce -ne $config.vsceVersion) {
    throw "package-lock.json pins vsce $lockedVsce, expected $($config.vsceVersion)."
}
if ($extensionPackage.version -notmatch '^\d+\.\d+\.\d+$') {
    throw "Unsupported extension version: $($extensionPackage.version)"
}

$goMod = Get-Content -LiteralPath (Join-Path $repoRoot 'go.mod') -Raw
$goModMatch = [regex]::Match($goMod, '(?m)^go\s+(\d+\.\d+\.\d+)\s*$')
if (-not $goModMatch.Success -or $goModMatch.Groups[1].Value -ne $config.goModVersion) {
    throw "go.mod must declare go $($config.goModVersion); refusing an unreviewed source/toolchain mismatch."
}
$goOutput = (& go version 2>&1 | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or $goOutput -notmatch '^go version go([0-9]+\.[0-9]+\.[0-9]+)\s') {
    throw "Unable to determine the Go toolchain version: $goOutput"
}
$actualGoVersion = $Matches[1]
if ($actualGoVersion -ne $config.goVersion) {
    throw "Pinned Go toolchain is $($config.goVersion); found $actualGoVersion. Install/select the pinned toolchain rather than silently changing the release build."
}
if ([version]$actualGoVersion -lt [version]$config.goModVersion) {
    throw "Go $actualGoVersion is older than the source requirement $($config.goModVersion)."
}

$nodeOutput = (& node --version 2>&1 | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or $nodeOutput -notmatch '^v(\d+)\.') {
    throw "Unable to determine the Node.js version: $nodeOutput"
}
if ([int]$Matches[1] -lt [int]$config.nodeMajor) {
    throw "Node.js $nodeOutput is below the required major version $($config.nodeMajor)."
}

$npmCache = Join-Path $repoRoot '.work\tools\npm-cache'
$npmArgs = @('ci', '--prefix', $toolsRoot, '--ignore-scripts', '--no-audit', '--no-fund', '--cache', $npmCache)
if ($Offline) { $npmArgs += '--offline' }
& npm.cmd @npmArgs
if ($LASTEXITCODE -ne 0) { throw "npm ci failed with exit code $LASTEXITCODE." }

$vsceEntry = Join-Path $toolsRoot 'node_modules\@vscode\vsce\vsce'
if (-not (Test-Path -LiteralPath $vsceEntry -PathType Leaf)) {
    throw "Pinned vsce entry point not found: $vsceEntry"
}
$vsceOutput = (& node $vsceEntry --version 2>&1 | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or $vsceOutput -ne $config.vsceVersion) {
    throw "Expected vsce $($config.vsceVersion), got '$vsceOutput'."
}

$outputRoot = [System.IO.Path]::GetFullPath($OutputDirectory)
if (-not (Test-Path -LiteralPath $outputRoot -PathType Container)) {
    New-Item -ItemType Directory -Path $outputRoot -Force | Out-Null
}
$vsixPath = Join-Path $outputRoot "intentlens-$($extensionPackage.version)-$Target.vsix"
if (Test-Path -LiteralPath $vsixPath) {
    throw "Refusing to overwrite an existing VSIX: $vsixPath"
}

$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("intentlens-release-" + [guid]::NewGuid().ToString('N'))
$stageRoot = Join-Path $tempRoot 'extension'
$cliPath = Join-Path $stageRoot 'bin\intentlens.exe'
$packageCreated = $false
New-Item -ItemType Directory -Path $stageRoot -Force | Out-Null
try {
    foreach ($item in Get-ChildItem -LiteralPath $extensionRoot -Force) {
        Copy-Item -LiteralPath $item.FullName -Destination $stageRoot -Recurse -Force
    }
    $binDirectory = Split-Path -Parent $cliPath
    New-Item -ItemType Directory -Path $binDirectory -Force | Out-Null

    $oldGoos = $env:GOOS
    $oldGoarch = $env:GOARCH
    $oldCgo = $env:CGO_ENABLED
    $oldGoToolchain = $env:GOTOOLCHAIN
    try {
        $env:GOOS = $config.target.goos
        $env:GOARCH = $config.target.goarch
        $env:CGO_ENABLED = '0'
        $env:GOTOOLCHAIN = 'local'
        Push-Location $repoRoot
        try {
            & go build -trimpath -buildvcs=false -o $cliPath ./cmd/intentlens
            if ($LASTEXITCODE -ne 0) { throw "Go build failed with exit code $LASTEXITCODE." }
        }
        finally {
            Pop-Location
        }
    }
    finally {
        $env:GOOS = $oldGoos
        $env:GOARCH = $oldGoarch
        $env:CGO_ENABLED = $oldCgo
        $env:GOTOOLCHAIN = $oldGoToolchain
    }

    $binaryInfo = (& go version -m $cliPath 2>&1 | Out-String)
    $versionPattern = "(?m):\s+go$([regex]::Escape($actualGoVersion))\s*$"
    if ($LASTEXITCODE -ne 0 -or $binaryInfo -notmatch "build\s+GOOS=$($config.target.goos)" -or $binaryInfo -notmatch "build\s+GOARCH=$($config.target.goarch)" -or $binaryInfo -notmatch $versionPattern) {
        throw "Built CLI does not report the expected Go version/target:`n$binaryInfo"
    }

    $goFiles = @(
        Get-Item -LiteralPath (Join-Path $repoRoot 'go.mod')
        Get-ChildItem -LiteralPath (Join-Path $repoRoot 'cmd') -Recurse -File -Filter '*.go'
        Get-ChildItem -LiteralPath (Join-Path $repoRoot 'internal') -Recurse -File -Filter '*.go'
    ) | Sort-Object { Get-RelativePath $repoRoot $_.FullName }
    $goFileRecords = @($goFiles | ForEach-Object {
        [pscustomobject]@{ path = (Get-RelativePath $repoRoot $_.FullName); sha256 = (Get-Sha256 $_.FullName) }
    })
    $goCanonical = (($goFileRecords | ForEach-Object { "$($_.path) $($_.sha256)" }) -join "`n") + "`n"
    $goSourceSha256 = Get-BytesSha256 ([System.Text.Encoding]::UTF8.GetBytes($goCanonical))

    $packageFiles = @(
        Get-ChildItem -LiteralPath $stageRoot -Recurse -File -Force |
            Where-Object {
                $relative = (Get-RelativePath $stageRoot $_.FullName).ToLowerInvariant()
                $relative -ne '.vscodeignore' -and
                $relative -ne 'release-manifest.json' -and
                -not $relative.StartsWith('test/') -and
                -not $relative.StartsWith('node_modules/') -and
                $relative -ne '.ds_store' -and
                -not $relative.EndsWith('.vsix')
            } |
            Sort-Object { Get-RelativePath $stageRoot $_.FullName }
    )
    $fileRecords = @($packageFiles | ForEach-Object {
        [pscustomobject]@{
            path = (Get-RelativePath $stageRoot $_.FullName).ToLowerInvariant()
            sha256 = (Get-Sha256 $_.FullName)
        }
    })

    $blockers = @()
    if ([string]::IsNullOrWhiteSpace($extensionPackage.publisher) -or $extensionPackage.publisher -eq 'intentlens-local') {
        $blockers += '发布前由项目所有者决定并填写正式的 VS Code Marketplace publisher ID。'
    }
    if ($extensionPackage.license -eq 'UNLICENSED' -or -not (Test-Path -LiteralPath (Join-Path $stageRoot 'LICENSE') -PathType Leaf)) {
        $blockers += '发布前由项目所有者选择合法许可证，并在清单与 LICENSE 文件中一致体现。'
    }

    $manifest = [ordered]@{
        schema = 1
        package = $extensionPackage.name
        version = $extensionPackage.version
        target = "$($config.target.goos)/$($config.target.goarch)"
        distributionReady = ($blockers.Count -eq 0)
        blockers = @($blockers)
        source = [ordered]@{
            goModVersion = $goModMatch.Groups[1].Value
            goVersion = $actualGoVersion
            goSourceSha256 = $goSourceSha256
            goFiles = $goFileRecords
        }
        packaging = [ordered]@{
            nodeVersion = $nodeOutput
            npmVersion = ((& npm.cmd --version 2>&1 | Out-String).Trim())
            vsceVersion = $vsceOutput
            vsceLockSha256 = (Get-Sha256 $toolsLockPath)
            vsceLockfileVersion = $lockInfo.version
        }
        files = $fileRecords
    }
    $manifestPath = Join-Path $stageRoot 'release-manifest.json'
    $manifestJson = $manifest | ConvertTo-Json -Depth 50
    [System.IO.File]::WriteAllText($manifestPath, $manifestJson + "`n", (New-Object System.Text.UTF8Encoding($false)))

    $vsceArgs = @('package', '--allow-missing-repository', '--target', $Target, '--out', $vsixPath)
    if ($extensionPackage.license -eq 'UNLICENSED') { $vsceArgs += '--skip-license' }
    Push-Location $stageRoot
    try {
        & node $vsceEntry @vsceArgs
        if ($LASTEXITCODE -ne 0) { throw "vsce package failed with exit code $LASTEXITCODE." }
    }
    finally {
        Pop-Location
    }
    $packageCreated = Test-Path -LiteralPath $vsixPath -PathType Leaf
    if (-not $packageCreated) { throw 'vsce reported success but did not create the expected package.' }

    $verifier = Join-Path $PSScriptRoot 'verify-vsix.ps1'
    $powershellExe = (Get-Process -Id $PID).Path
    & $powershellExe -NoLogo -NoProfile -ExecutionPolicy Bypass -File $verifier -Vsix $vsixPath -SourceExtension $stageRoot -RepositoryRoot $repoRoot
    if ($LASTEXITCODE -ne 0) { throw "VSIX integrity verification failed with exit code $LASTEXITCODE." }

    Write-Output "Candidate VSIX: $vsixPath"
    if (-not $manifest.distributionReady) {
        Write-Warning 'Package content passed integrity checks, but the manifest records unresolved owner decisions; this package is not cleared for public distribution.'
    }
}
catch {
    if ($packageCreated -and (Test-Path -LiteralPath $vsixPath)) {
        Remove-Item -LiteralPath $vsixPath -Force
    }
    throw
}
finally {
    if (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force
    }
}

