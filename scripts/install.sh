#!/usr/bin/env bash
set -euo pipefail

target="$(realpath "$(brew --prefix musup-go)/bin/musup-go")"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

go build -ldflags "-X github.com/toba/musup-go/cmd.ver=dev" -o "$tmp" .
install -m 755 "$tmp" "$target"

echo "Installed to $target"
