param()
$ErrorActionPreference = 'Stop'
$project = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$fixture = Join-Path ([IO.Path]::GetTempPath()) ("OKOKO test d'installation " + [guid]::NewGuid().ToString('N'))
$results = [Collections.Generic.List[object]]::new()
$oldTrace = $env:OKOKO_TEST_TRACE
$oldFailure = $env:OKOKO_TEST_FAILURE
$oldRoot = $env:OKOKO_INSTALL_ROOT
$oldRouter = $env:OKOKO_INSTALL_ROUTER
$oldNativeExit = $global:LASTEXITCODE
$oldSearchPath = $env:PATH
$runtime = if ($PSVersionTable.PSEdition -eq 'Desktop') { Join-Path $PSHOME 'powershell.exe' } elseif ($env:OS -eq 'Windows_NT') { Join-Path $PSHOME 'pwsh.exe' } else { Join-Path $PSHOME 'pwsh' }
function Check([bool]$Condition, [string]$Message) { if (-not $Condition) { throw $Message } }
function Scenario([string]$Name, [scriptblock]$Body) {
    Clear-Content -LiteralPath $env:OKOKO_TEST_TRACE
    $env:OKOKO_TEST_FAILURE = ''
    & $Body
    $results.Add([pscustomobject]@{name=$Name;status='passed'})
}
function Events { @((Get-Content -LiteralPath $env:OKOKO_TEST_TRACE) | Where-Object { $_ }) }
try {
    $scripts = Join-Path $fixture 'scripts'
    New-Item -ItemType Directory -Path $scripts -Force | Out-Null
    # Simulated deployment must not require a real SSH client, even in preparation-only mode.
    $stubBin = Join-Path $fixture 'bin'
    New-Item -ItemType Directory -Path $stubBin -Force | Out-Null
    foreach ($tool in @('ssh','scp')) {
        if ($env:OS -eq 'Windows_NT') {
            Set-Content -Encoding ASCII -LiteralPath (Join-Path $stubBin ($tool + '.cmd')) -Value '@exit /b 97'
        } else {
            $stub = Join-Path $stubBin $tool
            [IO.File]::WriteAllText($stub, "#!/bin/sh`nexit 97`n")
            & chmod +x $stub
            if ($LASTEXITCODE -ne 0) { throw 'Cannot prepare test command' }
        }
    }
    $env:PATH = $stubBin + [IO.Path]::PathSeparator + $oldSearchPath
    $env:OKOKO_TEST_TRACE = Join-Path $fixture 'trace.txt'
    Set-Content -LiteralPath $env:OKOKO_TEST_TRACE -Value ''
    Copy-Item -LiteralPath (Join-Path $project 'scripts/Install.ps1') -Destination $scripts
    @'
param()
Add-Content -LiteralPath $env:OKOKO_TEST_TRACE -Value 'test'
if ($env:OKOKO_TEST_FAILURE -eq 'test') { throw 'Test failure requested' }
'@ | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $scripts 'Test.ps1')
    @'
param([switch]$SkipTests)
if (-not $SkipTests) { throw 'Tests would run twice' }
Add-Content -LiteralPath $env:OKOKO_TEST_TRACE -Value 'build'
if ($env:OKOKO_TEST_FAILURE -eq 'build') { throw 'Build failure requested' }
'@ | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $scripts 'Build.ps1')
    @'
param([string]$Router, [switch]$UseExistingBuild)
if (-not $UseExistingBuild) { throw 'Build would run twice' }
if ($Router -ne '127.0.0.1') { throw 'Unexpected target' }
Add-Content -LiteralPath $env:OKOKO_TEST_TRACE -Value 'deploy'
if ($env:OKOKO_TEST_FAILURE -eq 'deploy') { throw 'Deploy failure requested' }
'@ | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $scripts 'Deploy.ps1')
    $install = Join-Path $scripts 'Install.ps1'
    Scenario 'single launcher, valid PowerShell syntax and Windows encoding' {
        Check (@(Get-ChildItem -LiteralPath $project -Filter '*.cmd').Count -eq 1) 'Expected one public CMD launcher'
        foreach ($file in Get-ChildItem -LiteralPath (Join-Path $project 'scripts') -Filter '*.ps1' -Recurse) {
            $tokens = $null; $issues = $null
            $ast = [Management.Automation.Language.Parser]::ParseFile($file.FullName, [ref]$tokens, [ref]$issues)
            Check (-not $issues) ("Invalid PowerShell syntax: " + $file.Name)
            $bytes = [IO.File]::ReadAllBytes($file.FullName)
            Check ($bytes.Length -ge 3 -and $bytes[0] -eq 239 -and $bytes[1] -eq 187 -and $bytes[2] -eq 191) ("Missing UTF-8 BOM: " + $file.Name)
            $policyCalls = $ast.FindAll({param($node) $node -is [Management.Automation.Language.CommandAst] -and $node.GetCommandName() -eq 'Set-ExecutionPolicy'}, $true)
            Check ($policyCalls.Count -eq 0) 'Execution policy must not be changed'
        }
    }
    Scenario 'preparation runs tests and build once without deployment' {
        & $install -PrepareOnly *> $null
        Check (((Events) -join ',') -eq 'test,build') 'Incorrect preparation sequence'
    }
    Scenario 'deployment follows tests and build, without duplicate work' {
        & $install -Router '127.0.0.1' *> $null
        Check (((Events) -join ',') -eq 'test,build,deploy') 'Incorrect deployment sequence'
    }
    foreach ($stage in @('test','build','deploy')) {
        Scenario ("failure stops the chain at " + $stage) {
            $env:OKOKO_TEST_FAILURE = $stage
            $failed = $false
            try { & $install -Router '127.0.0.1' *> $null } catch { $failed = $true }
            Check $failed 'Failure was swallowed'
            $expected = switch ($stage) { 'test' {'test'} 'build' {'test,build'} 'deploy' {'test,build,deploy'} }
            Check (((Events) -join ',') -eq $expected) 'Chain continued after an error'
        }
    }
    Scenario 'invalid target rejected before tests or deployment' {
        $failed = $false
        try { & $install -Router 'https://invalid.example' *> $null } catch { $failed = $true }
        Check ($failed -and (Events).Count -eq 0) 'Invalid target accepted'
    }
    $cmd = Get-Content -Raw -LiteralPath (Join-Path $project 'INSTALLER-OKOKO.cmd')
    $line = ($cmd -split "`n" | Where-Object { $_ -match '^powershell.exe ' })
    Check ($line -match '-Command "(?<body>.*)"\s*$') 'Missing inline bootstrap'
    $bootstrap = $Matches['body']
    Check ($bootstrap -notmatch 'Set-ExecutionPolicy|Bypass') 'Policy override found in bootstrap'
    $unblockMock = @'
function Unblock-File {
 [CmdletBinding()]
 param([Parameter(ValueFromPipeline=$true)]$InputObject)
 process {
  if ($env:OKOKO_TEST_FAILURE -eq 'unblock') { throw 'Unblock failure requested' }
  Add-Content -LiteralPath $env:OKOKO_TEST_TRACE -Value 'unblock'
 }
}
'@
    $env:OKOKO_INSTALL_ROOT = $fixture
    $env:OKOKO_INSTALL_ROUTER = '127.0.0.1'
    Scenario 'real bootstrap unblocks before loading scripts, including paths with spaces and apostrophes' {
        & $runtime -NoProfile -Command ($unblockMock + "`n" + $bootstrap) *> $null
        Check ($LASTEXITCODE -eq 0) 'Bootstrap failed'
        $events = Events
        Check ($events.Count -gt 3 -and ($events[-3..-1] -join ',') -eq 'test,build,deploy') 'Bootstrap sequence incorrect'
        Check (@($events[0..($events.Count-4)] | Where-Object { $_ -ne 'unblock' }).Count -eq 0) 'Script loaded before unblock'
    }
    Scenario 'unblock failure exits nonzero before running any script' {
        $env:OKOKO_TEST_FAILURE = 'unblock'
        & $runtime -NoProfile -Command ($unblockMock + "`n" + $bootstrap) *> $null
        Check ($LASTEXITCODE -ne 0 -and (Events).Count -eq 0) 'Unblock failure was ignored'
    }
    $report = [pscustomobject]@{powershell=$PSVersionTable.PSVersion.ToString();platform=$PSVersionTable.OS;windows_cmd_executed=$false;unblock='simulated';deployment='simulated';results=$results.ToArray()}
    $output = Join-Path $project 'test-results'
    New-Item -ItemType Directory -Path $output -Force | Out-Null
    $report | ConvertTo-Json -Depth 5 | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $output 'windows-launcher-tests.json')
    Write-Host ("Windows launcher: {0} checks passed (PowerShell {1}; platform: {2})." -f $results.Count, $PSVersionTable.PSVersion, $PSVersionTable.OS)
} finally {
    $env:OKOKO_TEST_TRACE = $oldTrace
    $env:OKOKO_TEST_FAILURE = $oldFailure
    $env:OKOKO_INSTALL_ROOT = $oldRoot
    $env:OKOKO_INSTALL_ROUTER = $oldRouter
    $env:PATH = $oldSearchPath
    $global:LASTEXITCODE = $oldNativeExit
    if (Test-Path -LiteralPath $fixture) { Remove-Item -LiteralPath $fixture -Recurse -Force }
}
