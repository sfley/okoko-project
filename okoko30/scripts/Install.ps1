param(
    [ValidatePattern('^$|^[A-Za-z0-9.-]+$')][string]$Router = '',
    [switch]$PrepareOnly
)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent

if ($PrepareOnly -and $Router) {
    throw 'Choisir soit un routeur, soit la preparation sans deploiement.'
}
if (-not $Router -and -not $PrepareOnly) {
    $Router = (Read-Host 'Adresse du routeur OpenWrt (Entree = tests et compilation seulement)').Trim()
}
if ($Router -and $Router -notmatch '^[A-Za-z0-9.-]+$') {
    throw 'Adresse du routeur invalide. Indiquer uniquement son adresse IP ou son nom, sans URL ni mot de passe.'
}

Write-Host '[2/5] Verification des outils' -ForegroundColor Cyan
$npmCommand = if ($env:OS -eq 'Windows_NT') { 'npm.cmd' } else { 'npm' }
$required = @('node', $npmCommand, 'go')
if ($Router) { $required += @('ssh', 'scp') }
foreach ($tool in $required) {
    if (-not (Get-Command $tool -CommandType Application -ErrorAction SilentlyContinue)) {
        throw "Outil absent : $tool. Installer Node.js >= 20.19, Go >= 1.24 et, pour le deploiement, le client OpenSSH Windows."
    }
}
$nodeVersion = & node -p 'process.versions.node'
if ($LASTEXITCODE -ne 0) { throw 'Impossible de determiner la version de Node.js.' }
$node = [version]$nodeVersion
if (-not (($node.Major -eq 20 -and $node -ge [version]'20.19.0') -or $node -ge [version]'22.12.0')) {
    throw 'Node.js 20.19+ (branche 20) ou 22.12+ est requis par Vite.'
}
$goVersion = & go version
if ($LASTEXITCODE -ne 0 -or $goVersion -notmatch '\bgo(\d+)\.(\d+)') {
    throw 'Impossible de determiner la version de Go.'
}
if ([version]("{0}.{1}" -f $Matches[1], $Matches[2]) -lt [version]'1.24') {
    throw 'Go 1.24 ou plus recent est requis.'
}
Write-Host "Projet : $root"
if ($Router) { Write-Host "Routeur : $Router" }

Write-Host '[3/5] Frontend, analyse et tests Go' -ForegroundColor Cyan
& "$PSScriptRoot/Test.ps1"

Write-Host '[4/5] Compilation Linux ARM64' -ForegroundColor Cyan
& "$PSScriptRoot/Build.ps1" -SkipTests

if ($Router) {
    Write-Host '[5/5] Transfert, sauvegarde et deploiement' -ForegroundColor Cyan
    & "$PSScriptRoot/Deploy.ps1" -Router $Router -UseExistingBuild
    Write-Host "Termine. Ouvrir http://${Router}:8888 et verifier les mesures de la chaudiere." -ForegroundColor Green
} else {
    Write-Host '[5/5] Preparation terminee, aucun deploiement demande' -ForegroundColor Green
    Write-Host 'Binaires disponibles dans build/. Pour deployer : INSTALLER-OKOKO.cmd ADRESSE_DU_ROUTEUR'
}
