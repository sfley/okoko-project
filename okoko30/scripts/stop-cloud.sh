#!/bin/sh
set -eu
PROJECT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
RUNTIME=/workspace/.okoko-runtime
if [ ! -s "$RUNTIME/server.pid" ]; then exit 0; fi
PID=$(cat "$RUNTIME/server.pid")
case "$PID" in ''|*[!0-9]*) echo 'PID invalide'; exit 1;; esac
if kill -0 "$PID" 2>/dev/null; then
 EXE=$(tr '\000' '\n' <"/proc/$PID/cmdline" 2>/dev/null | sed -n '1p')
 case "$EXE" in "$PROJECT/build/okoko30-linux-amd64") kill "$PID";; *) echo 'Processus étranger : aucun arrêt'; exit 1;; esac
 n=0; while kill -0 "$PID" 2>/dev/null && [ "$(tr '\000' '\n' <"/proc/$PID/cmdline" 2>/dev/null | sed -n '1p')" = "$PROJECT/build/okoko30-linux-amd64" ]; do n=$((n+1));if [ "$n" -ge 10 ];then echo 'Arrêt non confirmé';exit 1;fi;sleep 1;done
fi
rm -f "$RUNTIME/server.pid"
