param([switch]$Browser)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$npmCommand = if ($env:OS -eq 'Windows_NT') { 'npm.cmd' } else { 'npm' }
function Run([string]$Label, [scriptblock]$Command) { & $Command; if ($LASTEXITCODE -ne 0) { throw "$Label a échoué ($LASTEXITCODE)" } }
$oldOS = $env:GOOS; $oldARCH = $env:GOARCH
try {
 Remove-Item Env:GOOS -ErrorAction SilentlyContinue
 Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
 Push-Location "$root/frontend"
 try { Run 'Installation frontend' { & $npmCommand ci }; Run 'Tests des données météo' { & $npmCommand test }; Run 'Compilation frontend' { & $npmCommand run build } } finally { Pop-Location }
 Push-Location "$root/backend"
 try { Run 'Vérification modules' { go mod verify }; Run 'Analyse Go' { go vet ./... }; Run 'Tests Go' { go test -count=1 ./... } } finally { Pop-Location }
 & "$PSScriptRoot/tests/Install.Tests.ps1"
 if ($Browser) {
  Push-Location "$root/frontend"
  try { Run 'Tests navigateur' { & $npmCommand run test:browser } } finally { Pop-Location }
 }
 Write-Host 'Contrôles terminés. Les tests navigateur nécessitent une application lancée et Chromium.'
} finally { $env:GOOS = $oldOS; $env:GOARCH = $oldARCH }
