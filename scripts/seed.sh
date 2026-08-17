#!/usr/bin/env bash
# scripts/seed.sh — generates fixtures and writes them to the
# configured output directory.
set -euo pipefail
OUT="${OUT:-testdata/fixtures}"
PROFILE="${PROFILE:-medium}"

mkdir -p "$OUT"
go run ./cmd/seed -out "$OUT" -profile "$PROFILE"
