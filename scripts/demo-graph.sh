#!/usr/bin/env bash
# End-to-end demo of `rela graph`: build a tiny project with a mix of
# simple and hyphenated entity types, export DOT, and render to SVG via
# graphviz. Also demonstrates the filtered and LR-direction variants.

set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
BIN="${REPO}/bin/rela"
DEMO="$(mktemp -d -t rela-graph-demo.XXXXXX)"

cleanup() { rm -rf "${DEMO}"; }
trap cleanup EXIT

say() { printf '\n\033[1;34m==>\033[0m %s\n' "$*"; }
step() { printf '    %s\n' "$*"; }

say "Building rela → ${BIN}"
(cd "${REPO}" && go build -o "${BIN}" ./cmd/rela)

say "Seeding demo project at ${DEMO}"
cat > "${DEMO}/metamodel.yaml" <<'YAML'
version: "1.0"
namespace: "https://example.com/demo#"
types:
  review_response_status:
    values: [open, addressed]
    default: open
entities:
  feature:
    label: Feature
    id_prefix: "FEAT-"
    id_type: sequential
    color: "#E8F5E9"
    properties:
      title: {type: string, required: true}
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    id_type: sequential
    color: "#FFFDE7"
    properties:
      title: {type: string, required: true}
  review-response:
    label: Review response
    id_prefix: "RR-"
    id_type: sequential
    color: "#FFEBEE"
    properties:
      title: {type: string, required: true}
      status: {type: review_response_status}
relations:
  implements:
    label: implements
    from: [ticket]
    to: [feature]
  has-review-response:
    label: has review response
    from: [ticket]
    to: [review-response]
YAML

mkdir -p \
  "${DEMO}/entities/features" \
  "${DEMO}/entities/tickets" \
  "${DEMO}/entities/review-responses" \
  "${DEMO}/relations"

cat > "${DEMO}/entities/features/FEAT-001.md" <<'MD'
---
id: FEAT-001
type: feature
title: Graph DOT export
---
MD

cat > "${DEMO}/entities/tickets/TKT-001.md" <<'MD'
---
id: TKT-001
type: ticket
title: Render DOT with subgraph clusters per type
---
MD

cat > "${DEMO}/entities/review-responses/RR-001.md" <<'MD'
---
id: RR-001
type: review-response
title: Sanitize cluster IDs for hyphenated types
status: addressed
---
MD

cat > "${DEMO}/relations/TKT-001--implements--FEAT-001.md" <<'MD'
---
from: TKT-001
relation: implements
to: FEAT-001
---
MD

cat > "${DEMO}/relations/TKT-001--has-review-response--RR-001.md" <<'MD'
---
from: TKT-001
relation: has-review-response
to: RR-001
---
MD

cd "${DEMO}"

say "rela graph → stdout (DOT)"
"${BIN}" graph | sed -n '1,12p'
step "(truncated — first 12 lines shown)"

say "rela graph -o graph.dot  (write DOT to file)"
"${BIN}" graph -o graph.dot
step "wrote $(wc -l < graph.dot | tr -d ' ') lines to graph.dot"

if command -v dot >/dev/null 2>&1; then
    say "rela graph -o graph.svg -f svg  (render via graphviz)"
    "${BIN}" graph -o graph.svg -f svg
    step "SVG size: $(wc -c < graph.svg | tr -d ' ') bytes"

    say "rela graph -o graph-lr.png -f png --direction lr"
    "${BIN}" graph -o graph-lr.png -f png --direction lr
    step "PNG size: $(wc -c < graph-lr.png | tr -d ' ') bytes"

    say "rela graph --types ticket,feature -o tf.svg -f svg  (filtered)"
    "${BIN}" graph --types ticket,feature -o tf.svg -f svg
    step "filtered SVG size: $(wc -c < tf.svg | tr -d ' ') bytes"

    cp graph.svg graph-lr.png tf.svg /tmp/ 2>/dev/null || true
    say "Outputs copied to /tmp/graph.svg /tmp/graph-lr.png /tmp/tf.svg"
else
    say "graphviz 'dot' not found — skipping render steps"
    step "install with: brew install graphviz"
fi

say "Done."
