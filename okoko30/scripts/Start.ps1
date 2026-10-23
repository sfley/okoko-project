param([string]$Config = 'config.example.json')
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
Push-Location "$root/backend"
try {
 New-Item -ItemType Directory -Force data | Out-Null
 & go run ./cmd/chaudiere -config $Config
 if ($LASTEXITCODE -ne 0) { throw 'Démarrage échoué' }
} finally { Pop-Location }
