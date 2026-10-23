#!/bin/sh
set -eu
TOOLS=/workspace/.okoko-tools
mkdir -p "$TOOLS"
if [ ! -x "$TOOLS/go/bin/go" ]; then
 python3 - <<'PY'
from pathlib import Path
import urllib.request,hashlib,tarfile
root=Path('/workspace/.okoko-tools')
name='go1.27.3.linux-amd64.tar.gz'
base='https://dl.google.com/go/'
expected='65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40'
archive=root/name
with urllib.request.urlopen(base+name,timeout=60) as response,archive.open('wb') as target:
 while block:=response.read(1024*1024):target.write(block)
if hashlib.sha256(archive.read_bytes()).hexdigest()!=expected:raise SystemExit('Go archive checksum mismatch')
with tarfile.open(archive) as package:package.extractall(root,filter='data')
archive.unlink()
PY
fi
export PATH="$TOOLS/go/bin:$PATH"
export GOPATH="$TOOLS/gopath"
export GOCACHE="$TOOLS/go-cache"
cd /workspace/okoko-project/okoko30
mkdir -p build backend/data
npm ci --prefix frontend --cache "$TOOLS/npm-cache" --no-audit --no-fund
npm test --prefix frontend
npm run build --prefix frontend
cd backend
go mod download
go mod verify
go test -mod=readonly ./...
CGO_ENABLED=0 go build -mod=readonly -trimpath -ldflags='-s -w' -o ../build/okoko30-linux-amd64 ./cmd/chaudiere
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -mod=readonly -trimpath -ldflags='-s -w' -o ../build/okoko30-linux-arm64 ./cmd/chaudiere
