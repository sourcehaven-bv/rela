#!/usr/bin/env bash
# End-to-end demo of `rela schema --graphviz`: build a metamodel covering every
# classification bucket (plain, hub, legend-via-count, legend-via-connectedness),
# render through graphviz, and assert the PNG is non-empty. Also exercises
# --exclude.

set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
BIN="${REPO}/bin/rela"
DEMO="$(mktemp -d -t rela-schema-demo.XXXXXX)"

cleanup() { rm -rf "${DEMO}"; }
trap cleanup EXIT

say() { printf '\n\033[1;34m==>\033[0m %s\n' "$*"; }
step() { printf '    %s\n' "$*"; }

say "Building rela → ${BIN}"
(cd "${REPO}" && go build -o "${BIN}" ./cmd/rela)

say "Seeding a metamodel that hits every classification bucket"
# Buckets exercised:
#   plain:    core  → leaf  via "describes" (1 target)
#   hub:      hub   → {t1,t2,t3}, targets otherwise isolated (3 isolated)
#   legend-connected:  webber → {c1,c2,c3,c4}, targets also linked via anchor
#   legend-many:       catch  → {any..} (≥5 targets)
cat > "${DEMO}/metamodel.yaml" <<'YAML'
version: "1.0"
namespace: "https://example.com/demo#"
entities:
  core:
    label: Core
    id_prefix: "CORE-"
    id_type: sequential
    color: "#E3F2FD"
    properties:
      title: {type: string, required: true}
  leaf:
    label: Leaf
    id_prefix: "LEAF-"
    id_type: sequential
    color: "#E8F5E9"
    properties:
      title: {type: string, required: true}
  hub:
    label: Hub
    id_prefix: "HUB-"
    id_type: sequential
    color: "#FFF3E0"
    properties:
      title: {type: string, required: true}
  t1: {label: T1, id_prefix: "T1-", id_type: sequential, color: "#F3E5F5", properties: {title: {type: string, required: true}}}
  t2: {label: T2, id_prefix: "T2-", id_type: sequential, color: "#E0F7FA", properties: {title: {type: string, required: true}}}
  t3: {label: T3, id_prefix: "T3-", id_type: sequential, color: "#FCE4EC", properties: {title: {type: string, required: true}}}
  webber:
    label: Webber
    id_prefix: "WEB-"
    id_type: sequential
    color: "#FFFDE7"
    properties:
      title: {type: string, required: true}
  anchor:
    label: Anchor
    id_prefix: "ANC-"
    id_type: sequential
    color: "#EFEBE9"
    properties:
      title: {type: string, required: true}
  c1: {label: C1, id_prefix: "C1-", id_type: sequential, color: "#E3F2FD", properties: {title: {type: string, required: true}}}
  c2: {label: C2, id_prefix: "C2-", id_type: sequential, color: "#E8F5E9", properties: {title: {type: string, required: true}}}
  c3: {label: C3, id_prefix: "C3-", id_type: sequential, color: "#FFF3E0", properties: {title: {type: string, required: true}}}
  c4: {label: C4, id_prefix: "C4-", id_type: sequential, color: "#F3E5F5", properties: {title: {type: string, required: true}}}
  catch:
    label: Catch
    id_prefix: "CATCH-"
    id_type: sequential
    color: "#FFEBEE"
    properties:
      title: {type: string, required: true}
  u1: {label: U1, id_prefix: "U1-", id_type: sequential, color: "#E3F2FD", properties: {title: {type: string, required: true}}}
  u2: {label: U2, id_prefix: "U2-", id_type: sequential, color: "#E8F5E9", properties: {title: {type: string, required: true}}}
  u3: {label: U3, id_prefix: "U3-", id_type: sequential, color: "#FFF3E0", properties: {title: {type: string, required: true}}}
  u4: {label: U4, id_prefix: "U4-", id_type: sequential, color: "#F3E5F5", properties: {title: {type: string, required: true}}}
  u5: {label: U5, id_prefix: "U5-", id_type: sequential, color: "#E0F7FA", properties: {title: {type: string, required: true}}}
  excluded:
    label: Excluded
    id_prefix: "EX-"
    id_type: sequential
    color: "#F5F5F5"
    properties:
      title: {type: string, required: true}
relations:
  describes:
    label: describes
    from: [core]
    to: [leaf]
  fans:
    label: fans
    from: [hub]
    to: [t1, t2, t3]
  web:
    label: web
    from: [webber]
    to: [c1, c2, c3, c4]
  anchors:
    label: anchors
    from: [anchor]
    to: [c1, c2, c3, c4]
  catches:
    label: catches
    from: [catch]
    to: [u1, u2, u3, u4, u5]
  lost:
    label: lost
    from: [excluded]
    to: [leaf]
YAML

cd "${DEMO}"

say "rela schema --graphviz (DOT to stdout, first 20 lines)"
"${BIN}" schema --graphviz | sed -n '1,20p'
step "(truncated)"

check() {
    local label="$1"
    local out="$2"
    local pattern="$3"
    if printf '%s' "$out" | grep -qE "$pattern"; then
        step "✓ ${label}"
    else
        step "✗ ${label} (missing pattern: ${pattern})"
        echo "--- full DOT ---"
        printf '%s\n' "$out"
        exit 1
    fi
}
refute() {
    local label="$1"
    local out="$2"
    local pattern="$3"
    if printf '%s' "$out" | grep -qE "$pattern"; then
        step "✗ ${label} (unexpected pattern: ${pattern})"
        echo "--- full DOT ---"
        printf '%s\n' "$out"
        exit 1
    else
        step "✓ ${label}"
    fi
}

say "Structural assertions on the DOT output"
DOT="$("${BIN}" schema --graphviz)"
check "plain edge for 1-target relation" "$DOT" 'core -> leaf \[label="describes"'
check "hub bundle for 3 isolated targets" "$DOT" '__hub_[0-9]+ \[shape=point'
check "hub source edge labeled" "$DOT" 'hub -> __hub_[0-9]+ \[label="fans"'
check "legend node present" "$DOT" '__legend \[shape=plaintext'
check "legend mentions 'catches' (>=5 targets)" "$DOT" 'catches</I>'
check "legend mentions 'web' (3-4 connected)" "$DOT" 'web</I>'
refute "no direct edges for 'web' relation" "$DOT" 'webber -> c[0-9]+ \[label="web"'
refute "no direct edges for 'catches' relation" "$DOT" 'catch -> u[0-9]+ \[label="catches"'

say "Excluding an entity type"
EX_DOT="$("${BIN}" schema --graphviz --exclude excluded)"
refute "excluded node absent" "$EX_DOT" '  excluded \[label'
refute "no edges from excluded" "$EX_DOT" 'excluded -> '

say "Rendering to PNG via graphviz"
if ! command -v dot >/dev/null 2>&1; then
    say "graphviz 'dot' not found — skipping PNG render"
    step "install with: brew install graphviz  (or: apt-get install graphviz)"
    exit 0
fi

printf '%s\n' "$DOT" | dot -Tpng -o graph.png
SIZE=$(wc -c < graph.png | tr -d ' ')
if [ "$SIZE" -lt 1000 ]; then
    step "✗ PNG suspiciously small (${SIZE} bytes) — probably a render error"
    exit 1
fi
step "✓ PNG rendered (${SIZE} bytes)"

cp graph.png /tmp/rela-schema-demo.png 2>/dev/null || true
say "Output copied to /tmp/rela-schema-demo.png"

say "Done."
