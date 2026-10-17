#!/bin/sh
# Puts the browser build on the website: go/ (served at txhandles.github.io/pokemon-survivor/go/).
# The wasm is gzipped (about a quarter of the size); the page decompresses it in the browser.
# Run tools/build_go.sh first. CI runs this on every push to main and commits the result.
set -e
cd "$(dirname "$0")/.."
mkdir -p go
cp build/web/index.html build/web/wasm_exec.js go/
gzip -9 -n -c build/web/game.wasm > go/game.wasm.gz
rm -f go/game.wasm
ls -la go/
