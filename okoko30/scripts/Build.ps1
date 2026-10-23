param([switch]$SkipTests)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
if (-not $SkipTests) { & "$PSScriptRoot/Test.ps1" }
New-Item -ItemType Directory -Force "$root/build" | Out-Null
$oldOS = $env:GOOS; $oldARCH = $env:GOARCH; $oldCGO = $env:CGO_ENABLED
try {
 $env:GOOS = 'linux'; $env:GOARCH = 'arm64'; $env:CGO_ENABLED = '0'
 Push-Location "$root/backend"
 try {
  & go build -trimpath -ldflags='-s -w' -o "$root/build/okoko30-linux-arm64" ./cmd/chaudiere
  if ($LASTEXITCODE -ne 0) { throw 'Compilation ARM64 échouée' }
  & go build -trimpath -ldflags='-s -w' -o "$root/build/mariadb-import-linux-arm64" ./cmd/mariadb-import
  if ($LASTEXITCODE -ne 0) { throw "Compilation de l'outil de migration échouée" }
 } finally { Pop-Location }
 (Get-FileHash "$root/build/okoko30-linux-arm64" -Algorithm SHA256).Hash.ToLower() | Set-Content -Encoding ascii "$root/build/okoko30-linux-arm64.sha256"
 Write-Host 'Binaires Linux ARM64 disponibles dans build/. Aucun Node.js requis sur le routeur.'
} finally { $env:GOOS = $oldOS; $env:GOARCH = $oldARCH; $env:CGO_ENABLED = $oldCGO }
