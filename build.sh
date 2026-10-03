#!/bin/bash
set -euo pipefail

BIN_DIR="$HOME/iCloud/bin"
mkdir -p "$BIN_DIR"

# tssh client
go build -o "$BIN_DIR/tssh" .

# trz / tsz / trzsz — server-side file transfer tools (separate Go module)
for cmd in trz tsz trzsz; do
    go build -C third_party/trzsz-go -o "$BIN_DIR/$cmd" "./cmd/$cmd"
done
