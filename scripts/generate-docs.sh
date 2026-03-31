#!/bin/bash
# Generate documentation from rela entities using Lua scripts.
#
# This script:
# 1. Syncs the docs-project graph (validates entities/relations)
# 2. Runs the Lua script to generate docs from entities
# 3. Creates directories and ensures proper file endings
#
# Prerequisites:
#   - bin/rela must exist (run 'just build-cli' first)

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
RELA="$ROOT/bin/rela"
DOCS_PROJECT="$ROOT/docs-project"
OUTPUT="$ROOT/docs"

# Check prerequisites
if [[ ! -x "$RELA" ]]; then
    echo "ERROR: rela binary not found at $RELA"
    echo "Run 'just build-cli' first."
    exit 1
fi

echo "==> Syncing docs-project graph..."
(cd "$DOCS_PROJECT" && "$RELA" sync)

echo ""
echo "==> Validating docs-project..."
(cd "$DOCS_PROJECT" && "$RELA" analyze orphans) || true

# Create output directories
mkdir -p "$OUTPUT/tutorials"
mkdir -p "$OUTPUT/scenarios"

echo ""
echo "==> Generating documentation..."
(cd "$DOCS_PROJECT" && "$RELA" script "$ROOT/scripts/generate-docs.lua" --output-dir="$OUTPUT" docs)

echo ""
echo "==> Generating README.md..."
(cd "$DOCS_PROJECT" && "$RELA" script "$ROOT/scripts/generate-docs.lua" --output-dir="$ROOT" readme)

echo ""
echo "==> Done!"
