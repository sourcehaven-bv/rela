#!/usr/bin/env bash
# End-to-end demo of `rela acl audit`: seed a metamodel plus several acl.yaml
# policies (clean, un-gated membership, everyone-admin, schema drift) and assert
# the linter reports the right findings and gates its exit code on --fail-on.
# Doubles as a smoke test for the CLI wiring (load → audit → render → exit).

set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
BIN="${REPO}/bin/rela"
DEMO="$(mktemp -d -t rela-acl-audit-demo.XXXXXX)"

cleanup() { rm -rf "${DEMO}"; }
trap cleanup EXIT

say() { printf '\n\033[1;34m==>\033[0m %s\n' "$*"; }
step() { printf '    %s\n' "$*"; }

say "Building rela → ${BIN}"
(cd "${REPO}" && go build -o "${BIN}" ./cmd/rela)

# A shared metamodel for every policy below: person/group/ticket, plus two
# candidate membership relations (member-of, heeft_rol).
seed_metamodel() {
    local dir="$1"
    mkdir -p "${dir}"
    cat > "${dir}/schema.yaml" <<'YAML'
version: "1"
namespace: "https://example.com/acl-demo#"
entities:
  person:
    label: Person
    id_prefix: "PERS-"
    id_type: sequential
    properties:
      name: {type: string, required: true}
  group:
    label: Group
    id_prefix: "GRP-"
    id_type: sequential
    properties:
      name: {type: string, required: true}
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    id_type: sequential
    properties:
      title: {type: string, required: true}
      status: {type: string, values: [open, doing, done]}
relations:
  member-of:
    label: member of
    from: [person]
    to: [group]
  heeft_rol:
    label: heeft rol
    from: [person]
    to: [group]
YAML
}

# run_audit <dir> [flags...] → prints output, returns the audit's exit code.
run_audit() {
    local dir="$1"; shift
    "${BIN}" --project "${dir}" acl audit "$@"
}

# expect_rc <label> <expected-rc> <dir> [flags...]
expect_rc() {
    local label="$1" want="$2" dir="$3"; shift 3
    local got=0
    run_audit "${dir}" "$@" >/dev/null 2>&1 || got=$?
    if [ "${got}" -eq "${want}" ]; then
        step "✓ ${label} (exit ${got})"
    else
        step "✗ ${label}: exit ${got}, want ${want}"
        run_audit "${dir}" "$@" || true
        exit 1
    fi
}

# expect_finding <label> <dir> <rule-id>
expect_finding() {
    local label="$1" dir="$2" rule="$3"
    # Capture, then match with a here-string — never `... | grep -q`.
    #
    # `grep -q` exits the instant it matches. If the writer is still writing it
    # dies of SIGPIPE (141), and `set -o pipefail` surfaces that as a failed
    # pipeline — so the assertion fails BECAUSE the finding was found. Whether
    # it fires depends on the output fitting the 64K pipe buffer, which is why
    # this passed locally and went red only in CI, where the audit logs more.
    #
    # Capturing alone is not enough: `printf '%s' "$out" | grep -q` has exactly
    # the same race. The pipe has to go.
    local out
    out="$(run_audit "${dir}" 2>&1 || true)"
    if grep -q "${rule}" <<<"${out}"; then
        step "✓ ${label} (${rule})"
    else
        step "✗ ${label}: expected finding ${rule}"
        run_audit "${dir}" || true
        exit 1
    fi
}

# refute_finding <label> <dir> <rule-id>
refute_finding() {
    local label="$1" dir="$2" rule="$3"
    # Same capture-then-match reasoning as expect_finding.
    local out
    out="$(run_audit "${dir}" 2>&1 || true)"
    if grep -q "${rule}" <<<"${out}"; then
        step "✗ ${label}: unexpected finding ${rule}"
        run_audit "${dir}" || true
        exit 1
    else
        step "✓ ${label} (no ${rule})"
    fi
}

# ---------------------------------------------------------------------------
say "1. Clean, well-gated policy → no findings, exits 0 at every threshold"
CLEAN="${DEMO}/clean"
seed_metamodel "${CLEAN}"
cat > "${CLEAN}/acl.yaml" <<'YAML'
user_entity_type: person
roles:
  admin:
    create: [ticket]
    update: [ticket]
    delete: [ticket]
    read: [ticket]
    permissions: [delegate-membership]
  everyone:                 # read-only everyone is a legitimate pattern
    read: ["*"]
assignments:
  ops-team: admin
role_relations:
  member-of:
    requires_permission: delegate-membership
YAML
run_audit "${CLEAN}"
expect_rc "clean: advisory"        0 "${CLEAN}"
expect_rc "clean: --fail-on=any"   0 "${CLEAN}" --fail-on=any

# ---------------------------------------------------------------------------
say "2. Un-gated membership relation → A1 (high)"
UNGATED="${DEMO}/ungated"
seed_metamodel "${UNGATED}"
cat > "${UNGATED}/acl.yaml" <<'YAML'
roles:
  editor:
    create: [ticket]
    update: [ticket]
    delete: [ticket]
    read: [ticket]
assignments:
  engineering: editor
YAML
run_audit "${UNGATED}" || true
expect_finding "un-gated member-of flagged" "${UNGATED}" "A1-ungated-membership"
expect_rc "ungated: --fail-on=high fails CI" 1 "${UNGATED}" --fail-on=high

# ---------------------------------------------------------------------------
say "3. Privileged 'everyone' role → A3 (critical); read-only everyone does not"
EVERYONE="${DEMO}/everyone"
seed_metamodel "${EVERYONE}"
cat > "${EVERYONE}/acl.yaml" <<'YAML'
roles:
  everyone:
    update: [ticket]
    read: [ticket]
YAML
run_audit "${EVERYONE}" || true
expect_finding "write-granting everyone flagged" "${EVERYONE}" "A3-everyone-privileged"
refute_finding "read-only everyone NOT flagged" "${CLEAN}" "A3-everyone-privileged"

# ---------------------------------------------------------------------------
say "4. Schema drift → B1 (undeclared type) + B2 (undeclared relation) + A4"
DRIFT="${DEMO}/drift"
seed_metamodel "${DRIFT}"
cat > "${DRIFT}/acl.yaml" <<'YAML'
membership_relation: heeft_roll     # typo: heeft_rol
roles:
  editor:
    create: [tickets]               # typo: ticket
    read: [tickets]
assignments:
  engineering: edutor               # typo: editor
YAML
run_audit "${DRIFT}" || true
expect_finding "undeclared type flagged"     "${DRIFT}" "B1-undeclared-type"
expect_finding "undeclared relation flagged" "${DRIFT}" "B2-undeclared-relation"
expect_finding "assignment to unknown role"  "${DRIFT}" "A4-assignment-unknown-role"

# ---------------------------------------------------------------------------
say "5. --fail-on threshold: medium-only findings gate as expected"
WARN="${DEMO}/warn"
seed_metamodel "${WARN}"
cat > "${WARN}/acl.yaml" <<'YAML'
roles:                              # A9 wildcard write (medium) is the worst
  power:
    create: ["*"]
    update: ["*"]
    delete: ["*"]
    read: ["*"]
YAML
run_audit "${WARN}" || true
expect_rc "medium-only: advisory"       0 "${WARN}"
expect_rc "medium-only: --fail-on=high" 0 "${WARN}" --fail-on=high
expect_rc "medium-only: --fail-on=medium fails" 1 "${WARN}" --fail-on=medium
expect_rc "medium-only: --fail-on=any fails"    1 "${WARN}" --fail-on=any

# ---------------------------------------------------------------------------
say "6. JSON output → AnalysisResult envelope"
JSON="$(run_audit "${EVERYONE}" -o json)"
printf '%s\n' "${JSON}"
if grep -q '"status": "warning"' <<<"${JSON}" \
    && grep -q '"rule": "A3-everyone-privileged"' <<<"${JSON}"; then
    step "✓ JSON envelope carries status + rule"
else
    step "✗ JSON envelope missing expected fields"
    exit 1
fi

# ---------------------------------------------------------------------------
say "7. No acl.yaml → nothing to audit, exits 0"
NONE="${DEMO}/none"
seed_metamodel "${NONE}"   # metamodel only, no acl.yaml
run_audit "${NONE}"
expect_rc "no policy: --fail-on=any still exits 0" 0 "${NONE}" --fail-on=any

say "Done — all acl audit assertions passed."
