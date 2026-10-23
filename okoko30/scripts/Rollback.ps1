param([Parameter(Mandatory=$true)][ValidatePattern('^[A-Za-z0-9.-]+$')][string]$Router,[Parameter(Mandatory=$true)][ValidatePattern('^/[A-Za-z0-9_./-]+$')][string]$BackupDir,[ValidatePattern('^[A-Za-z0-9_-]+$')][string]$User='root',[ValidatePattern('^/[A-Za-z0-9_./-]+$')][string]$AppPath='/root/chaudiere-go',[ValidatePattern('^/[A-Za-z0-9_./-]+$')][string]$ConfigPath='/root/config.json')
$ErrorActionPreference='Stop'
$script=@'
set -eu
BACK=$1
APP=$2
APP_CONFIG=$3
test -s "$BACK/chaudiere-go"
if [ -x /etc/init.d/okoko ]; then /etc/init.d/okoko stop; fi
for p in $(pidof "$(basename "$APP")" 2>/dev/null || true); do target=$(readlink /proc/$p/exe 2>/dev/null || true); case "$target" in "$APP"|"$APP (deleted)") kill "$p";; esac; done
sleep 3
cp "$BACK/chaudiere-go" "$APP.previous"
chmod 755 "$APP.previous"
mv -f "$APP.previous" "$APP"
if [ -x /etc/init.d/okoko ]; then /etc/init.d/okoko start; else cd "$(dirname "$APP_CONFIG")"; "$APP" -config "$APP_CONFIG" >/dev/null 2>&1 </dev/null & fi
echo 'Binaire précédent restauré. Base SQLite et configuration actuelles conservées.'
'@
$script | & ssh "${User}@${Router}" "tr -d '\r' | sh -s -- '$BackupDir' '$AppPath' '$ConfigPath'"
if ($LASTEXITCODE -ne 0) { throw 'Retour arrière échoué' }
