#!/bin/bash
# Generate documentation from rela entities using mdcomp templates.
#
# This script:
# 1. Syncs the docs-project graph (validates entities/relations)
# 2. Renders each entity through mdcomp templates to produce docs/
# 3. Generates README.md from the readme template
#
# Prerequisites:
#   - bin/rela must exist (run 'make build' first)
#   - mdcomp must be installed (uv tool install mdcomp)

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
RELA="$ROOT/bin/rela"
DOCS_PROJECT="$ROOT/docs-project"
TEMPLATES="$ROOT/docs/templates"
OUTPUT="$ROOT/docs"

# Check prerequisites
if [[ ! -x "$RELA" ]]; then
    echo "ERROR: rela binary not found at $RELA"
    echo "Run 'just build-cli' first."
    exit 1
fi

if ! command -v mdcomp &> /dev/null; then
    echo "ERROR: mdcomp not found in PATH"
    echo "Install with: uv tool install mdcomp"
    exit 1
fi

echo "==> Syncing docs-project graph..."
(cd "$DOCS_PROJECT" && "$RELA" sync)

echo ""
echo "==> Validating docs-project..."
(cd "$DOCS_PROJECT" && "$RELA" analyze orphans) || true

# Helper: render template and ensure trailing newline
render() {
    local template="$1"
    local output="$2"
    shift 2
    mdcomp render "$template" "$@" -o "$output"
    # Ensure file ends with a newline (mdcomp strips trailing whitespace)
    if [ -s "$output" ] && [ "$(tail -c 1 "$output" | wc -l)" -eq 0 ]; then
        echo "" >> "$output"
    fi
}

echo ""
echo "==> Generating guide pages..."
for entity_file in "$DOCS_PROJECT"/entities/guide/*.md; do
    [[ -f "$entity_file" ]] || continue
    filename=$(basename "$entity_file" .md)
    slug="${filename#GUIDE-}"
    output_file="$OUTPUT/$slug.md"

    render "$TEMPLATES/guide.md.j2" "$output_file" \
        --var "entity_file=$entity_file" \
        --content-base "$ROOT"

    echo "  $slug.md"
done

echo ""
echo "==> Generating tutorial pages..."
mkdir -p "$OUTPUT/tutorials"
for entity_file in "$DOCS_PROJECT"/entities/tutorial/*.md; do
    [[ -f "$entity_file" ]] || continue
    filename=$(basename "$entity_file" .md)
    slug="${filename#TUT-}"
    output_file="$OUTPUT/tutorials/$slug.md"

    render "$TEMPLATES/tutorial.md.j2" "$output_file" \
        --var "entity_file=$entity_file" \
        --content-base "$ROOT"

    echo "  tutorials/$slug.md"
done

echo ""
echo "==> Generating scenario pages..."
mkdir -p "$OUTPUT/scenarios"
for entity_file in "$DOCS_PROJECT"/entities/scenario/*.md; do
    [[ -f "$entity_file" ]] || continue
    filename=$(basename "$entity_file" .md)
    slug="${filename#SCN-}"
    output_file="$OUTPUT/scenarios/$slug.md"

    render "$TEMPLATES/scenario.md.j2" "$output_file" \
        --var "entity_file=$entity_file" \
        --content-base "$ROOT"

    echo "  scenarios/$slug.md"
done

echo ""
echo "==> Generating README.md..."
render "$TEMPLATES/readme.md.j2" "$ROOT/README.md" \
    --content-base "$ROOT"
echo "  README.md"

echo ""
echo "==> Done! Generated docs from $(ls "$DOCS_PROJECT"/entities/*/*.md | wc -l | tr -d ' ') entities."
