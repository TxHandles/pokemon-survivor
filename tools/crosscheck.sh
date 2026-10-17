#!/bin/sh
# Checks that the simulation is bit-identical across build targets (needed for lockstep co-op):
# runs cmd/simhash natively and as wasm under Node and compares the state-hash traces.
set -e
cd "$(dirname "$0")/.."
out=${TMPDIR:-/tmp}/ps-crosscheck
mkdir -p "$out"
go build -o "$out/simhash" ./cmd/simhash
GOOS=js GOARCH=wasm go build -o "$out/simhash.wasm" ./cmd/simhash
wasmdir="$(go env GOROOT)/lib/wasm"
[ -d "$wasmdir" ] || wasmdir="$(go env GOROOT)/misc/wasm"
cp "$wasmdir/wasm_exec.js" "$wasmdir/wasm_exec_node.js" "$out/"
"$out/simhash" 300 > "$out/native.txt"
(cd "$out" && env -i PATH="$PATH" node wasm_exec_node.js simhash.wasm 300 > wasm.txt)
if cmp -s "$out/native.txt" "$out/wasm.txt"; then
  echo "simulation identical on native and wasm: $(tail -n1 "$out/native.txt")"
else
  echo "SIMULATION DIFFERS between native and wasm builds:"; diff "$out/native.txt" "$out/wasm.txt" | head; exit 1
fi
