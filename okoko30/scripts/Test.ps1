param([switch]$Browser)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
function Run([string]$Label, [scriptblock]$Command) { & $Command; if ($LASTEXITCODE -ne 0) { throw "$Label a échoué ($LASTEXITCODE)" } }
$oldOS = $env:GOOS; $oldARCH = $env:GOARCH
try {
 Remove-Item Env:GOOS -ErrorAction SilentlyContinue
 Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
 Push-Location "$root/frontend"
 try { Run 'Installation frontend' { npm ci }; Run 'Tests des données météo' { npm test }; Run 'Compilation frontend' { npm run build } } finally { Pop-Location }
 Push-Location "$root/backend"
 try { Run 'Vérification modules' { go mod verify }; Run 'Analyse Go' { go vet ./... }; Run 'Tests Go' { go test -count=1 ./... } } finally { Pop-Location }
 if ($Browser) {
  Push-Location "$root/frontend"
  try { Run 'Tests navigateur' { npm run test:browser } } finally { Pop-Location }
 }
 Write-Host 'Contrôles terminés. Les tests navigateur nécessitent une application lancée et Chromium.'
} finally { $env:GOOS = $oldOS; $env:GOARCH = $oldARCH }
