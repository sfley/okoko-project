#!/bin/sh
# Start only this checkout's local development instance; no real boiler URL is required.
set -eu
PROJECT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
RUNTIME=/workspace/.okoko-runtime
BINARY="$PROJECT/build/okoko30-linux-amd64"
mkdir -p "$RUNTIME" "$PROJECT/backend/data"
chmod 700 "$RUNTIME"
if [ -s "$RUNTIME/server.pid" ]; then
 PID=$(cat "$RUNTIME/server.pid")
 case "$PID" in ''|*[!0-9]*) echo 'Fichier PID invalide'; exit 1;; esac
 if kill -0 "$PID" 2>/dev/null; then
  EXE=$(tr '\000' '\n' <"/proc/$PID/cmdline" 2>/dev/null | sed -n '1p')
  case "$EXE" in "$BINARY") echo "OKOKO déjà démarré (PID $PID)"; exit 0;; *) echo 'Le PID appartient à un autre processus ; aucun arrêt effectué'; exit 1;; esac
 fi
fi
if curl --fail --silent --max-time 1 http://127.0.0.1:8888/health >/dev/null 2>&1; then echo 'Port 8888 déjà occupé ; aucun autre service arrêté'; exit 1; fi
test -x "$BINARY" || { echo 'Compiler le backend avant le démarrage'; exit 1; }
cd "$PROJECT/backend"
CONFIG=${1:-config.example.json}
nohup "$BINARY" -config "$CONFIG" >"$RUNTIME/server.log" 2>&1 </dev/null &
PID=$!
echo "$PID" >"$RUNTIME/server.pid"
sleep 1
kill -0 "$PID" 2>/dev/null || { echo "Le processus n'a pas démarré ; consulter $RUNTIME/server.log"; exit 1; }
n=0
until STATUS=$(curl --fail --silent --max-time 2 http://127.0.0.1:8888/health) && [ "$STATUS" = ok ]; do
 n=$((n+1)); if [ "$n" -ge 10 ] || ! kill -0 "$PID" 2>/dev/null; then echo "Démarrage non confirmé ; consulter $RUNTIME/server.log"; exit 1; fi
 sleep 1
done
curl --fail --silent --max-time 3 http://127.0.0.1:8888/api/v3/snapshot >/dev/null
echo "OKOKO prêt pour le développement local (PID $PID)."
