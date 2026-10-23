#!/bin/sh
# Password-free deployment. Existing SSH authentication and configuration stay on the router.
set -eu
APP=$1
APP_CONFIG=$2
APP_DB=$3
UPLOAD=$4
EXPECTED=$5
PORT=$6
SERVICE=/etc/init.d/okoko
for p in "$APP" "$APP_CONFIG" "$APP_DB" "$UPLOAD"; do
 case "$p" in /*) ;; *) echo 'Chemin absolu requis'; exit 1;; esac
 case "$p" in *[!a-zA-Z0-9_./-]*|*/../*) echo 'Chemin non pris en charge'; exit 1;; esac
done
command -v sqlite3 >/dev/null || { echo 'sqlite3 requis pour la sauvegarde cohérente'; exit 1; }
command -v sha256sum >/dev/null || { echo 'sha256sum requis'; exit 1; }
test -s "$APP" && test -s "$APP_CONFIG" && test -s "$UPLOAD"
test "$(sha256sum "$UPLOAD" | cut -d ' ' -f 1)" = "$EXPECTED" || { echo 'Empreinte du transfert invalide'; exit 1; }
STAMP=$(date +%Y%m%d-%H%M%S)
BACK="$(dirname "$APP")/okoko-rollback-$STAMP"
umask 077
mkdir "$BACK"
cp -p "$APP" "$BACK/chaudiere-go"
cp -p "$APP_CONFIG" "$BACK/config.json"
if [ -x "$SERVICE" ]; then cp -p "$SERVICE" "$BACK/okoko.init"; fi
pids() {
 for p in $(pidof "$(basename "$APP")" 2>/dev/null || true); do
  target=$(readlink "/proc/$p/exe" 2>/dev/null || true)
  case "$target" in "$APP"|"$APP (deleted)") echo "$p";; esac
 done
}
start_app() {
 if [ -x "$SERVICE" ]; then "$SERVICE" start; else
  cd "$(dirname "$APP_CONFIG")"
  "$APP" -config "$APP_CONFIG" >>"$BACK/start.log" 2>&1 </dev/null &
 fi
}
stop_app() {
 if [ -x "$SERVICE" ]; then "$SERVICE" stop; fi
 P=$(pids); if [ -n "$P" ]; then kill $P; fi
 n=0; while [ -n "$(pids)" ] && [ "$n" -lt 20 ]; do sleep 1; n=$((n+1)); done
 test -z "$(pids)"
}
INSTALLED=0
STOPPED=0
rollback_on_error() {
 status=$?
 trap - EXIT INT TERM
 if [ "$status" -ne 0 ] && [ "$STOPPED" = 1 ]; then
  echo 'Échec : remise en place du binaire précédent, base actuelle conservée.'
  stop_app || true
  if [ "$INSTALLED" = 1 ]; then cp "$BACK/chaudiere-go" "$APP.restore"; chmod 755 "$APP.restore"; mv -f "$APP.restore" "$APP"; fi
  start_app || true
  echo "Copie SQLite de sécurité : $BACK/chaudiere.db"
 fi
 exit "$status"
}
trap rollback_on_error EXIT INT TERM
STOPPED=1
stop_app || { echo 'Arrêt impossible, aucun remplacement du binaire'; exit 1; }
if [ -f "$APP_DB" ]; then
 sqlite3 "$APP_DB" ".backup '$BACK/chaudiere.db'"
 test "$(sqlite3 "$BACK/chaudiere.db" 'PRAGMA quick_check;')" = ok || { echo 'Sauvegarde invalide'; exit 1; }
fi
chmod 755 "$UPLOAD"
mv "$UPLOAD" "$APP.next"
INSTALLED=1
mv -f "$APP.next" "$APP"
start_app
n=0
until wget -q -T 2 -O /dev/null "http://127.0.0.1:$PORT/health"; do
 n=$((n+1)); if [ "$n" -ge 15 ]; then echo 'Service non prêt'; exit 1; fi
 sleep 1
done
wget -q -T 3 -O /dev/null "http://127.0.0.1:$PORT/api/v3/snapshot"
STOPPED=0
trap - EXIT INT TERM
echo "OKOKO 3.0 installé. Retour arrière disponible dans : $BACK"
echo 'La base existante et la configuration locale ont été conservées.'
