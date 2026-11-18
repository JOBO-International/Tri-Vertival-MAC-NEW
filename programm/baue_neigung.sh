#!/bin/bash
# Baut das Zusatzprogramm Vertical3-Neigung.exe
set -e
cd /home/claude/v3go
export GOPROXY=direct GOSUMDB=off GOFLAGS=-mod=mod
VER=${1:-1.0.0.0}
gofmt -w cmd/vertical3neigung neigung profil
go run ./tools/mkres cmd/vertical3neigung/rsrc_windows_amd64.syso icon.ico $VER "Tri-Vertical Kameraneigung – Zusatzprogramm"
CGO_CFLAGS="-DLIBDE265_STATIC_BUILD" CGO_CXXFLAGS="-DLIBDE265_STATIC_BUILD" CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
  CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
  go build -ldflags '-H windowsgui -s -w -extldflags "-static"' -o /home/claude/build/Vertical3-Neigung.exe ./cmd/vertical3neigung 2>&1 \
  | grep -vE "warning|libde265|In file included|^\s+\||^\s+[0-9]+ \||from |note:|~|\^" || true
ls -la /home/claude/build/Vertical3-Neigung.exe
