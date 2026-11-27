#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
exec python3 "$ROOT_DIR/scripts/build.py" binaries --output "$HOME/OneDrive/bin"
