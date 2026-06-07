#!/bin/bash
set -euo pipefail

mkdir -p "$HOME/iCloud/bin"
go build -o "$HOME/iCloud/bin/tssh" .
