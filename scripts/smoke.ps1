param([Parameter(Mandatory=$true)][string]$WorkDirectory)
$ErrorActionPreference = 'Stop'
$project = Split-Path -Parent $PSScriptRoot
$binary = Join-Path $project 'bin\intentlens.exe'
if (-not (Test-Path -LiteralPath $binary)) { throw 'Build bin/intentlens.exe first' }
$fixture = Join-Path ([System.IO.Path]::GetFullPath($WorkDirectory)) ('smoke-' + [guid]::NewGuid().ToString('N'))
$repo = Join-Path $fixture 'repo'
$state = Join-Path $fixture 'state'
New-Item -ItemType Directory -Path $repo,$state -Force | Out-Null
$utf8 = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText((Join-Path $repo 'go.mod'),"module smoke`n`ngo 1.24.0`n",$utf8)
[System.IO.File]::WriteAllText((Join-Path $repo 'main.go'),"package smoke`nfunc Twice(n int) int { return n }`n",$utf8)
[System.IO.File]::WriteAllText((Join-Path $repo '.env'),"PRIVATE_FILE_SENTINEL",$utf8)
$start = @{hook_event_name='UserPromptSubmit';session_id='smoke-session';cwd=$repo;prompt='PRIVATE_PROMPT_SENTINEL';transcript_path='unused'} | ConvertTo-Json -Compress
$start | & $binary hook --state $state
if ($LASTEXITCODE -ne 0) { throw 'Start hook blocked' }
[System.IO.File]::WriteAllText((Join-Path $repo 'main.go'),"package smoke`nfunc Twice(n int) int { return n * 2 }`n",$utf8)
(@{hook_event_name='Stop';session_id='smoke-session';cwd=$repo} | ConvertTo-Json -Compress) | & $binary hook --state $state
if ($LASTEXITCODE -ne 0) { throw 'Stop hook blocked' }
$rounds = & $binary rounds --root $repo --state $state | ConvertFrom-Json
if ($LASTEXITCODE -ne 0 -or @($rounds).Count -ne 1 -or @($rounds[0].changes).Count -ne 1) { throw 'Net changes incorrect' }
$inspection = '{"file":"main.go","line":2}' | & $binary inspect --root $repo --state $state | ConvertFrom-Json
if ($LASTEXITCODE -ne 0 -or $inspection.function.name -ne 'Twice') { throw 'Function inspection failed' }
Get-ChildItem -LiteralPath $state -Recurse -File | ForEach-Object {
 if ([System.IO.File]::ReadAllText($_.FullName) -match 'PRIVATE_(FILE|PROMPT)_SENTINEL') { throw 'Privacy regression' }
}
$result = @{result='PASS';fixture=$fixture;rounds=@($rounds).Count;changes=@($rounds[0].changes).Count;function=$inspection.function.name;model_calls=0;checks=@('official hook-shaped inputs','start baseline to stop net change','excluded .env','prompt not persisted','Go function evidence')}
$result | ConvertTo-Json -Depth 5
