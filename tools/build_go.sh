#!/bin/sh
# Builds the Go version: build/windows/PokemonSurvivor.exe and build/web/ (index.html + game.wasm + wasm_exec.js).
# Needs Go 1.24+. Run from the repo root.
set -e
export GOTOOLCHAIN=local
mkdir -p build/windows build/web
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w -H=windowsgui" -o build/windows/PokemonSurvivor.exe ./cmd/game
GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o build/web/game.wasm ./cmd/game
cp web/index.html build/web/
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" build/web/ 2>/dev/null || cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" build/web/
echo "built build/windows/PokemonSurvivor.exe and build/web/"
