@echo off
setlocal
set "OKOKO_INSTALL_ROOT=%~dp0"
set "OKOKO_INSTALL_FILE=%~f0"
set "OKOKO_INSTALL_ROUTER=%~1"
powershell.exe -NoLogo -NoProfile -NoExit -Command "$ErrorActionPreference = 'Stop'; try { $text = [IO.File]::ReadAllText($env:OKOKO_INSTALL_FILE); $marker = ':OKOKO_' + 'POWERSHELL'; $position = $text.IndexOf($marker); if ($position -lt 0) { throw 'Script incomplet' }; & ([scriptblock]::Create($text.Substring($position + $marker.Length))); $global:LASTEXITCODE = 0 } catch { $global:LASTEXITCODE = 1; Write-Host ('ERREUR : ' + $_.Exception.Message) -ForegroundColor Red; Write-Host 'Etapes suivantes annulees. Relancez ce script apres correction.' -ForegroundColor Yellow }; Write-Host 'La fenetre reste ouverte. Fermez-la vous-meme quand vous avez termine.' -ForegroundColor Cyan"
if errorlevel 1 pause
exit /b
:OKOKO_POWERSHELL
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath($env:OKOKO_INSTALL_ROOT)
Set-Location -LiteralPath $root
Write-Host 'OKOKO - installation avec reprise automatique' -ForegroundColor Cyan
foreach ($required in @('frontend/package-lock.json','backend/go.mod','scripts/remote-deploy.sh')) {
    if (-not (Test-Path -LiteralPath (Join-Path $root $required))) {
        throw 'Placer ce script dans le dossier OKOKO contenant frontend, backend et scripts.'
    }
}
Write-Host '[1/6] Deblocage des fichiers, sans changement de strategie PowerShell'
Get-ChildItem -LiteralPath $root -Recurse -File | Unblock-File -ErrorAction Stop
$router = [string]$env:OKOKO_INSTALL_ROUTER
if ($router -eq '--prepare') { $router = '' }
elseif (-not $router) { $router = (Read-Host 'Adresse du routeur (Entree = preparation sans deploiement)').Trim() }
if ($router -and $router -notmatch '^[A-Za-z0-9.-]+$') {
    throw 'Indiquer uniquement une adresse IP ou un nom de routeur, sans URL ni mot de passe.'
}
$npm = if ($env:OS -eq 'Windows_NT') { 'npm.cmd' } else { 'npm' }
$tools = @('node', $npm, 'go')
if ($router) { $tools += @('ssh','scp') }
foreach ($tool in $tools) {
    if (-not (Get-Command $tool -CommandType Application -ErrorAction SilentlyContinue)) {
        throw "Outil absent : $tool. Installer Node.js, Go et, pour deployer, le client OpenSSH."
    }
}
$nodeVersion = & node -p 'process.versions.node'
if ($LASTEXITCODE -ne 0) { throw 'Lecture de la version Node.js impossible.' }
$node = [version]$nodeVersion
if (-not (($node.Major -eq 20 -and $node -ge [version]'20.19.0') -or $node -ge [version]'22.12.0')) {
    throw 'Node.js 20.19+ (branche 20) ou 22.12+ requis.'
}
$npmVersion = & $npm --version
if ($LASTEXITCODE -ne 0) { throw 'Lecture de la version npm impossible.' }
$goVersion = & go version
if ($LASTEXITCODE -ne 0 -or $goVersion -notmatch '\bgo(\d+)\.(\d+)') { throw 'Version Go illisible.' }
if ([version]("{0}.{1}" -f $Matches[1],$Matches[2]) -lt [version]'1.24') { throw 'Go 1.24 minimum requis.' }
$scriptHash = (Get-FileHash -LiteralPath $env:OKOKO_INSTALL_FILE -Algorithm SHA256).Hash
$stateFile = Join-Path $root 'test-results/install-resume.json'
$state = @{}
if (Test-Path -LiteralPath $stateFile) {
    try {
        $saved = Get-Content -LiteralPath $stateFile -Raw | ConvertFrom-Json
        foreach ($property in $saved.PSObject.Properties) { $state[$property.Name] = $property.Value }
    } catch { Write-Host 'Etat de reprise illisible : les etapes seront verifiees a nouveau.' -ForegroundColor Yellow }
}
function Fingerprint([string[]]$Paths, [switch]$Required) {
    $entries = [Collections.Generic.List[string]]::new()
    foreach ($relative in $Paths) {
        $path = Join-Path $root $relative
        if (-not (Test-Path -LiteralPath $path)) {
            if ($Required) { throw "Resultat attendu absent : $relative" }
            $entries.Add("absent:$relative")
            continue
        }
        $files = if (Test-Path -LiteralPath $path -PathType Container) { @(Get-ChildItem -LiteralPath $path -Recurse -File -Force | Sort-Object FullName) } else { @(Get-Item -LiteralPath $path -Force) }
        $entries.Add("path:$relative")
        foreach ($file in $files) {
            $name = $file.FullName.Substring($root.Length).Replace('\','/')
            $hash = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash
            $entries.Add("${name}:$hash")
        }
    }
    $algorithm = [Security.Cryptography.SHA256]::Create()
    try { return ([BitConverter]::ToString($algorithm.ComputeHash([Text.Encoding]::UTF8.GetBytes(($entries -join "`n"))))).Replace('-','').ToLowerInvariant() }
    finally { $algorithm.Dispose() }
}
function Save-State {
    New-Item -ItemType Directory -Force (Split-Path $stateFile -Parent) | Out-Null
    $state | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath ($stateFile + '.next') -Encoding UTF8
    Move-Item -LiteralPath ($stateFile + '.next') -Destination $stateFile -Force
}
function Native([string]$Label, [scriptblock]$Action) {
    & $Action
    if ($LASTEXITCODE -ne 0) { throw "$Label : echec (code $LASTEXITCODE)." }
}
function Step([string]$Id, [string]$Label, [string]$InputHash, [string[]]$Artifacts, [scriptblock]$Action) {
    $old = $state[$Id]
    $current = Fingerprint $Artifacts
    if ($old -and $old.input -eq $InputHash -and $old.output -eq $current) {
        Write-Host "$Label - DEJA OK, etape ignoree" -ForegroundColor Green
        return
    }
    $state.Remove($Id)
    Save-State
    Write-Host "$Label - en cours" -ForegroundColor Cyan
    & $Action
    $state[$Id] = @{input=$InputHash; output=(Fingerprint $Artifacts -Required); completed_at=[DateTime]::UtcNow.ToString('o')}
    Save-State
    Write-Host "$Label - OK" -ForegroundColor Green
}
$frontendPaths = @('frontend/package.json','frontend/package-lock.json','frontend/index.html','frontend/vite.config.js','frontend/src','frontend/public')
$frontendHash = Fingerprint $frontendPaths
$dependenciesKey = "$scriptHash|$nodeVersion|$npmVersion|$(Fingerprint @('frontend/package.json','frontend/package-lock.json'))"
Step 'dependencies' '[2/6] Dependances frontend' $dependenciesKey @('frontend/node_modules/.package-lock.json','frontend/node_modules/react/package.json','frontend/node_modules/vite/package.json') {
    Push-Location (Join-Path $root 'frontend')
    try { Native 'npm ci' { & $npm ci } } finally { Pop-Location }
}
$frontendKey = "$scriptHash|$nodeVersion|$npmVersion|$frontendHash|$(Fingerprint @('frontend/tests'))"
Step 'frontend-tests' '[3/6] Tests frontend' $frontendKey @() {
    Push-Location (Join-Path $root 'frontend')
    try { Native 'Tests frontend' { & $npm test } } finally { Pop-Location }
}
Step 'frontend-build' '[3/6] Compilation frontend' $frontendKey @('backend/internal/web/dist/index.html','backend/internal/web/dist/assets') {
    Push-Location (Join-Path $root 'frontend')
    try { Native 'Compilation frontend' { & $npm run build } } finally { Pop-Location }
}
$backendHash = Fingerprint @('backend/go.mod','backend/go.sum','backend/cmd','backend/internal/app','backend/internal/web/web.go')
$distHash = Fingerprint @('backend/internal/web/dist') -Required
$goKey = "$scriptHash|$goVersion|$backendHash|$distHash"
$oldGoOS = $env:GOOS; $oldGoArch = $env:GOARCH; $oldCGO = $env:CGO_ENABLED
try {
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    $env:CGO_ENABLED = '0'
    Step 'go-tests' '[4/6] Verification et tests Go' $goKey @() {
        Push-Location (Join-Path $root 'backend')
        try {
            Native 'Verification modules Go' { & go mod verify }
            Native 'Analyse Go' { & go vet ./... }
            Native 'Tests Go' { & go test -count=1 ./... }
        } finally { Pop-Location }
    }
    Step 'arm64' '[5/6] Compilation ARM64' $goKey @('build/okoko30-linux-arm64','build/mariadb-import-linux-arm64','build/okoko30-linux-arm64.sha256') {
        $env:GOOS = 'linux'; $env:GOARCH = 'arm64'
        New-Item -ItemType Directory -Force (Join-Path $root 'build') | Out-Null
        Push-Location (Join-Path $root 'backend')
        try {
            Native 'Compilation application ARM64' { & go build -trimpath -ldflags='-s -w' -o (Join-Path $root 'build/okoko30-linux-arm64') ./cmd/chaudiere }
            Native 'Compilation outil ARM64' { & go build -trimpath -ldflags='-s -w' -o (Join-Path $root 'build/mariadb-import-linux-arm64') ./cmd/mariadb-import }
        } finally { Pop-Location }
        (Get-FileHash -LiteralPath (Join-Path $root 'build/okoko30-linux-arm64') -Algorithm SHA256).Hash.ToLowerInvariant() | Set-Content -Encoding ASCII -LiteralPath (Join-Path $root 'build/okoko30-linux-arm64.sha256')
    }
} finally { $env:GOOS = $oldGoOS; $env:GOARCH = $oldGoArch; $env:CGO_ENABLED = $oldCGO }
if ($router) {
    $target = "root@$router"
    $binary = Join-Path $root 'build/okoko30-linux-arm64'
    $hash = (Get-FileHash -LiteralPath $binary -Algorithm SHA256).Hash.ToLowerInvariant()
    Write-Host '[6/6] Verification de la version deja installee sur le routeur' -ForegroundColor Cyan
    $probe = @'
APP=/root/chaudiere-go
ready=0
if test -s "$APP" && command -v sha256sum >/dev/null && command -v wget >/dev/null; then
 disk=$(sha256sum "$APP" | cut -d ' ' -f 1)
 for p in $(pidof chaudiere-go 2>/dev/null || true); do
  active=$(readlink "/proc/$p/exe" 2>/dev/null || true)
  if test "$active" = "$APP"; then
   running=$(sha256sum "/proc/$p/exe" 2>/dev/null | cut -d ' ' -f 1)
   if test "$running" = "$disk"; then ready=1; break; fi
  fi
 done
 if test "$ready" = 1 && wget -q -T 3 -O /dev/null http://127.0.0.1:8888/health && wget -q -T 3 -O /dev/null http://127.0.0.1:8888/api/v3/snapshot; then
  printf 'OKOKO_HASH=%s\n' "$disk"
 else
  echo OKOKO_DEPLOY_REQUIRED
 fi
else
 echo OKOKO_DEPLOY_REQUIRED
fi
'@
    $probeOutput = @($probe | & ssh -o ConnectTimeout=10 -o ConnectionAttempts=1 $target "tr -d '\r' | sh -s")
    if ($LASTEXITCODE -ne 0) { throw 'Connexion SSH impossible. Aucun deploiement lance.' }
    if ($probeOutput -contains "OKOKO_HASH=$hash") {
        Write-Host '[6/6] Deploiement DEJA OK : meme binaire actif et application disponible' -ForegroundColor Green
    } else {
        Write-Host '[6/6] Sauvegarde et deploiement' -ForegroundColor Cyan
        $upload = '/root/chaudiere-go.upload-' + [guid]::NewGuid().ToString('N')
        Native 'Transfert' { & scp -O $binary "${target}:$upload" }
        $remote = (Get-Content -LiteralPath (Join-Path $root 'scripts/remote-deploy.sh') -Encoding UTF8 -Raw).Replace("`r`n","`n")
        Native 'Sauvegarde et deploiement' { $remote | & ssh -o ConnectTimeout=10 -o ConnectionAttempts=1 $target "tr -d '\r' | sh -s -- '/root/chaudiere-go' '/root/config.json' '/root/chaudiere/chaudiere.db' '$upload' '$hash' '8888'" }
        Write-Host "Installation terminee : http://${router}:8888" -ForegroundColor Green
    }
} else {
    Write-Host '[6/6] Aucun deploiement demande. Binaires disponibles dans build/.' -ForegroundColor Green
}
Write-Host 'Termine. Les etapes validees seront ignorees a la prochaine execution si leurs fichiers sont inchanges.' -ForegroundColor Green
