param()
$ErrorActionPreference = 'Stop'
$project = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$entry = Join-Path $project 'INSTALLER-OKOKO.cmd'
$text = [IO.File]::ReadAllText($entry)
$marker = ':OKOKO_' + 'POWERSHELL'
$body = $text.Substring($text.IndexOf($marker) + $marker.Length)
$fixture = Join-Path ([IO.Path]::GetTempPath()) ("OKOKO reprise d'installation " + [guid]::NewGuid().ToString('N'))
$results = [Collections.Generic.List[object]]::new()
$variables = @('OKOKO_INSTALL_ROOT','OKOKO_INSTALL_FILE','OKOKO_INSTALL_ROUTER','OKOKO_TEST_LOG','OKOKO_TEST_FAIL','OKOKO_TEST_REMOTE','PATH','GOOS','GOARCH','CGO_ENABLED')
$saved = @{}
foreach ($name in $variables) { $saved[$name] = [Environment]::GetEnvironmentVariable($name) }
$oldExit = $global:LASTEXITCODE
$oldLocation = Get-Location
function Check([bool]$Condition,[string]$Message) { if (-not $Condition) { throw $Message } }
function Record([string]$Message) { Add-Content -LiteralPath $env:OKOKO_TEST_LOG -Value $Message }
function Actions { @(Get-Content -LiteralPath $env:OKOKO_TEST_LOG | Where-Object { $_ -and $_ -notmatch ':version$' }) }
function Scenario([string]$Name,[scriptblock]$Action) {
    Clear-Content -LiteralPath $env:OKOKO_TEST_LOG
    $env:OKOKO_TEST_FAIL = ''
    & $Action
    $results.Add([pscustomobject]@{name=$Name;status='passed'})
}
function Run-Payload { & ([scriptblock]::Create($body)) *> $null }
function Unblock-File {
    [CmdletBinding()]
    param([Parameter(ValueFromPipeline=$true)]$InputObject)
    process { if ($env:OKOKO_TEST_FAIL -eq 'unblock') { throw 'Unblock failure requested' } }
}
function node { $global:LASTEXITCODE=0; Record 'node:version'; '24.19.0' }
function Invoke-NpmStub {
    $global:LASTEXITCODE=0
    if ($args[0] -eq '--version') { Record 'npm:version'; '11.9.0'; return }
    $operation = $args -join ' '
    Record "npm:$operation"
    if ($env:OKOKO_TEST_FAIL -eq $operation) { $global:LASTEXITCODE=37; return }
    if ($operation -eq 'ci') {
        foreach ($file in @('node_modules/.package-lock.json','node_modules/react/package.json','node_modules/vite/package.json')) {
            $path = Join-Path (Join-Path $env:OKOKO_INSTALL_ROOT 'frontend') $file
            New-Item -ItemType Directory -Force (Split-Path $path -Parent) | Out-Null
            Set-Content -LiteralPath $path -Value '{}'
        }
    }
    if ($operation -eq 'run build') {
        $dist = Join-Path $env:OKOKO_INSTALL_ROOT 'backend/internal/web/dist'
        New-Item -ItemType Directory -Force (Join-Path $dist 'assets') | Out-Null
        $source = Get-Content -Raw -LiteralPath (Join-Path $env:OKOKO_INSTALL_ROOT 'frontend/src/value.js')
        Set-Content -LiteralPath (Join-Path $dist 'index.html') -Value $source
        Set-Content -LiteralPath (Join-Path $dist 'assets/app.js') -Value $source
    }
}
function go {
    $global:LASTEXITCODE=0
    if ($args[0] -eq 'version') { Record 'go:version'; 'go version go1.27.3 linux/amd64'; return }
    Record ('go:' + ($args -join ' '))
    if ($env:OKOKO_TEST_FAIL -eq 'go') { $global:LASTEXITCODE=38; return }
    if ($args[0] -eq 'build') {
        $position = [Array]::IndexOf($args,'-o')
        Check ($position -ge 0) 'Missing Go output path'
        Set-Content -LiteralPath $args[$position+1] -Value 'BINARY FOR TESTS ONLY'
    }
}
function ssh {
    $incoming = ($input | Out-String)
    $global:LASTEXITCODE=0
    if ($incoming -match 'OKOKO_DEPLOY_REQUIRED') {
        Record 'ssh:probe'
        if ($env:OKOKO_TEST_FAIL -eq 'ssh') { $global:LASTEXITCODE=255; return }
        if ($env:OKOKO_TEST_REMOTE -eq 'same') {
            'OKOKO_HASH=' + (Get-FileHash -LiteralPath (Join-Path $env:OKOKO_INSTALL_ROOT 'build/okoko30-linux-arm64') -Algorithm SHA256).Hash.ToLowerInvariant()
        } else { 'OKOKO_DEPLOY_REQUIRED' }
    } else { Record 'ssh:deploy' }
}
function scp { Record 'scp:upload'; $global:LASTEXITCODE=0 }
try {
    foreach ($file in @('frontend/package.json','frontend/package-lock.json','frontend/index.html','frontend/vite.config.js','frontend/src/value.js','frontend/tests/value.test.js','backend/go.mod','backend/go.sum','backend/cmd/main.go','backend/internal/app/app.go','backend/internal/web/web.go','scripts/remote-deploy.sh')) {
        $path = Join-Path $fixture $file
        New-Item -ItemType Directory -Force (Split-Path $path -Parent) | Out-Null
        Set-Content -LiteralPath $path -Value 'FIXTURE ONLY'
    }
    $bin = Join-Path $fixture 'bin'
    New-Item -ItemType Directory -Force $bin | Out-Null
    foreach ($tool in @('ssh','scp')) {
        if ($env:OS -eq 'Windows_NT') { Set-Content -LiteralPath (Join-Path $bin ($tool+'.cmd')) -Value '@exit /b 97' }
        else { $path=Join-Path $bin $tool; [IO.File]::WriteAllText($path,"#!/bin/sh`nexit 97`n"); & chmod +x $path }
    }
    $env:PATH = $bin + [IO.Path]::PathSeparator + $saved['PATH']
    $env:OKOKO_INSTALL_ROOT = $fixture + [IO.Path]::DirectorySeparatorChar
    $env:OKOKO_INSTALL_FILE = $entry
    $env:OKOKO_INSTALL_ROUTER = '--prepare'
    $env:OKOKO_TEST_LOG = Join-Path $fixture 'trace.txt'
    Set-Content -LiteralPath $env:OKOKO_TEST_LOG -Value ''
    $npmName = if ($env:OS -eq 'Windows_NT') { 'npm.cmd' } else { 'npm' }
    Set-Item -Path ("Function:\" + $npmName) -Value ${function:Invoke-NpmStub}
    Scenario 'single launcher keeps PowerShell open and contains no policy override' {
        Check ($text -match '^@echo off') 'Missing CMD entry'
        $line = $text -split "`n" | Where-Object { $_ -match '^powershell.exe ' }
        Check ($line -match '-NoExit\b' -and $line -notmatch 'Set-ExecutionPolicy|Bypass') 'Window/policy settings incorrect'
        Check ($line -match '-Command "(?<command>.*)"\s*$') 'Missing bootstrap'
        foreach ($source in @($Matches['command'],$body)) {
            $tokens=$null; $issues=$null
            [Management.Automation.Language.Parser]::ParseInput($source,[ref]$tokens,[ref]$issues) | Out-Null
            Check (-not $issues) 'Invalid PowerShell syntax'
        }
        Check ($line -notmatch '\bexit\s+[01]\b') 'Bootstrap would close the window'
    }
    Scenario 'first run performs the preparation steps' {
        Run-Payload
        $events=Actions
        Check (@($events | Where-Object {$_ -eq 'npm:ci'}).Count -eq 1) 'Dependencies not installed once'
        Check (@($events | Where-Object {$_ -eq 'npm:test'}).Count -eq 1) 'Frontend tests not run once'
        Check (@($events | Where-Object {$_ -like 'go:build *'}).Count -eq 2) 'Both binaries must be built'
    }
    Scenario 'second run skips all unchanged successful stages' { Run-Payload; Check (@(Actions).Count -eq 0) 'Successful steps repeated' }
    Scenario 'source changes invalidate dependent steps but preserve dependencies' {
        Add-Content -LiteralPath (Join-Path $fixture 'frontend/src/value.js') -Value 'changed'
        Run-Payload; $events=Actions
        Check ($events -notcontains 'npm:ci' -and $events -contains 'npm:test' -and $events -contains 'npm:run build') 'Frontend invalidation incorrect'
        Check (@($events | Where-Object {$_ -like 'go:test *'}).Count -eq 1) 'Changed bundle must be tested'
    }
    Scenario 'corrupted binary rebuilds ARM64 without repeating tests' {
        Add-Content -LiteralPath (Join-Path $fixture 'build/okoko30-linux-arm64') -Value 'corruption'
        Run-Payload; $events=Actions
        Check ($events.Count -eq 2 -and @($events | Where-Object {$_ -like 'go:build *'}).Count -eq 2) 'Artifact corruption not detected'
    }
    Scenario 'failed stage stops the chain and is not marked successful' {
        Add-Content -LiteralPath (Join-Path $fixture 'frontend/src/value.js') -Value 'changed again'
        $env:OKOKO_TEST_FAIL='test'; $failed=$false
        try { Run-Payload } catch { $failed=$true }
        Check ($failed -and ((Actions) -join ',') -eq 'npm:test') 'Chain continued after failure'
        $state=Get-Content -Raw -LiteralPath (Join-Path $fixture 'test-results/install-resume.json') | ConvertFrom-Json
        Check ($null -eq $state.'frontend-tests') 'Failed stage was checkpointed'
    }
    Scenario 'restart retries the failed stage while skipping completed dependencies' {
        Run-Payload; $events=Actions
        Check ($events -notcontains 'npm:ci' -and $events -contains 'npm:test' -and $events -contains 'npm:run build') 'Resume incorrect'
    }
    Scenario 'missing installed dependency invalidates installation checkpoint' {
        Remove-Item -LiteralPath (Join-Path $fixture 'frontend/node_modules/react/package.json')
        Run-Payload; Check (((Actions) -join ',') -eq 'npm:ci') 'Missing dependency not detected'
    }
    Scenario 'SSH failure stops before transfer or deployment' {
        $env:OKOKO_INSTALL_ROUTER='127.0.0.1'; $env:OKOKO_TEST_FAIL='ssh'; $failed=$false
        try { Run-Payload } catch { $failed=$true }
        Check ($failed -and ((Actions) -join ',') -eq 'ssh:probe') 'SSH failure ignored'
    }
    Scenario 'verified identical active remote binary skips deployment' {
        $env:OKOKO_TEST_REMOTE='same'; Run-Payload
        Check (((Actions) -join ',') -eq 'ssh:probe') 'Already deployed binary was transferred'
    }
    Scenario 'different remote binary triggers one transfer and deployment' {
        $env:OKOKO_TEST_REMOTE='different'; Run-Payload
        Check (((Actions) -join ',') -eq 'ssh:probe,scp:upload,ssh:deploy') 'Deployment sequence incorrect'
    }
    Scenario 'unblock failure stops before any tool is run' {
        $env:OKOKO_TEST_FAIL='unblock'; $failed=$false
        try { Run-Payload } catch { $failed=$true }
        Check ($failed -and @(Actions).Count -eq 0) 'Unblock failure ignored'
    }
    $output=Join-Path $project 'test-results'
    New-Item -ItemType Directory -Force $output | Out-Null
    [pscustomobject]@{powershell=$PSVersionTable.PSVersion.ToString();platform=$PSVersionTable.OS;windows_cmd_executed=$false;unblock='simulated';native_tools='simulated';deployment='simulated';results=$results.ToArray()} | ConvertTo-Json -Depth 6 | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $output 'windows-launcher-tests.json')
    Write-Host ("Windows launcher: {0} checks passed; resume, artifact validation and stop-on-error verified (PowerShell {1}, Linux/Windows console behavior not executed)." -f $results.Count,$PSVersionTable.PSVersion)
} finally {
    Set-Location -LiteralPath $oldLocation.Path
    foreach ($name in $variables) { [Environment]::SetEnvironmentVariable($name,$saved[$name]) }
    $global:LASTEXITCODE=$oldExit
    if (Test-Path -LiteralPath $fixture) { Remove-Item -LiteralPath $fixture -Recurse -Force }
}
