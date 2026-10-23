param(
 [Parameter(Mandatory=$true)][ValidatePattern('^[A-Za-z0-9.-]+$')][string]$Router,
 [ValidatePattern('^[A-Za-z0-9_-]+$')][string]$User = 'root',
 [ValidatePattern('^/[A-Za-z0-9_./-]+$')][string]$AppPath = '/root/chaudiere-go',
 [ValidatePattern('^/[A-Za-z0-9_./-]+$')][string]$ConfigPath = '/root/config.json',
 [ValidatePattern('^/[A-Za-z0-9_./-]+$')][string]$DatabasePath = '/root/chaudiere/chaudiere.db',
 [ValidateRange(1,65535)][int]$Port = 8888,
 [switch]$UseExistingBuild
)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
if (-not $UseExistingBuild) { & "$PSScriptRoot/Build.ps1" }
$binary = "$root/build/okoko30-linux-arm64"
if (-not (Test-Path $binary)) { throw "Binaire absent : compiler d'abord." }
$hash = (Get-FileHash $binary -Algorithm SHA256).Hash.ToLower()
$upload = "$AppPath.upload-$([guid]::NewGuid().ToString('N'))"
$target = "${User}@${Router}"
& scp -O $binary "${target}:$upload"
if ($LASTEXITCODE -ne 0) { throw 'Transfert échoué ; ancienne version conservée.' }
$script = (Get-Content -Encoding UTF8 -Raw "$PSScriptRoot/remote-deploy.sh").Replace("`r`n","`n")
$script | & ssh $target "tr -d '\r' | sh -s -- '$AppPath' '$ConfigPath' '$DatabasePath' '$upload' '$hash' '$Port'"
if ($LASTEXITCODE -ne 0) { throw 'Déploiement échoué. Lire le résultat de la restauration automatique.' }
Write-Host 'Déploiement terminé. Contrôlez les mesures réelles et conservez le dossier de retour arrière annoncé.'
