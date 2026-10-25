#!/usr/bin/env bash
# Regenerate every file the gauntlet benchmarks from its source, with the
# compiler at HEAD. The list is gauntlet/regen.txt; the regeneration is
# cmd/check's (cmd/check/gauntlet.go), the same one its gauntlet step runs to
# require the committed copies to be byte-identical.
#
#   bash gauntlet/regen.sh           # write into the gauntlet
#   bash gauntlet/regen.sh DIR       # write under DIR instead, to compare
set -euo pipefail
cd "$(dirname "$0")/.."
exec go run ./cmd/check -regen "${1:-.}"
