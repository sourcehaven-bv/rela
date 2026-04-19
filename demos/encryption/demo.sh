#!/usr/bin/env bash
# End-to-end demo for the rela at-rest encryption feature.
#
# Walks through the full lifecycle:
#   1. Create a cleartext rela project with sample requirements.
#   2. Generate age identities for alice, bob, eve.
#   3. Encrypt the project for alice only; verify bob cannot read.
#   4. Add bob as a recipient; verify bob can now read.
#   5. Remove bob; verify bob has lost access to future content.
#   6. Eve (never a recipient) never has access.
#   7. Decrypt the project back to cleartext; verify contents preserved.
#
# The script returns 0 only if every expected invariant holds.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
RELA="${REPO_ROOT}/bin/rela"

step() { printf '\n=== %s ===\n' "$1"; }
pass() { printf '  \033[32mOK\033[0m  %s\n' "$1"; }
fail() { printf '  \033[31mFAIL\033[0m %s\n' "$1"; exit 1; }

# Build rela if missing.
if [[ ! -x "${RELA}" ]]; then
    step "Building rela binary"
    (cd "${REPO_ROOT}" && go build -o "${RELA}" ./cmd/rela)
fi

# Work in a fresh tempdir.
WORK="$(mktemp -d -t rela-enc-demo.XXXXXX)"
trap 'rm -rf "${WORK}"' EXIT
cd "${WORK}"

step "1. Init cleartext rela project"
PROJ="${WORK}/demo-project"
mkdir -p "${PROJ}"
(cd "${PROJ}" && "${RELA}" init) >/dev/null
cd "${PROJ}"
# The default metamodel uses legacy syntax; run migrate so subsequent
# commands work.
"${RELA}" migrate >/dev/null 2>&1 || true
pass "project initialized at ${PROJ}"

step "2. Create a couple of requirements + a decision + a relation in cleartext"
"${RELA}" create req --id REQ-001 --title "Public roadmap" --status proposed >/dev/null
"${RELA}" create req --id REQ-002 --title "Confidential: rotate master API key" --status proposed >/dev/null
"${RELA}" create decision --id DEC-001 --title "Rotate quarterly" --status proposed >/dev/null
"${RELA}" link DEC-001 addresses REQ-002 >/dev/null

REL_FILE="relations/DEC-001--addresses--REQ-002.md"

if ! grep -q "Confidential" entities/requirements/REQ-002.md; then
    fail "expected cleartext body in entities/requirements/REQ-002.md"
fi
if ! grep -q "from: DEC-001" "${REL_FILE}"; then
    fail "expected cleartext relation in ${REL_FILE}"
fi
pass "on-disk entity and relation files are human-readable cleartext"

step "3. Generate age identities for alice, bob, eve"
KEYS_WORK="${WORK}/keys-work"
mkdir -p "${KEYS_WORK}"
"${RELA}" keys generate alice --out "${KEYS_WORK}" >/dev/null
"${RELA}" keys generate bob   --out "${KEYS_WORK}" >/dev/null
"${RELA}" keys generate eve   --out "${KEYS_WORK}" >/dev/null
for who in alice bob eve; do
    [[ -s "${KEYS_WORK}/${who}.pub" ]] || fail "${who}.pub not written"
    [[ -s "${KEYS_WORK}/${who}.key" ]] || fail "${who}.key not written"
done
pass "alice/bob/eve identity + pubkey files generated"

step "4. Encrypt the project for alice (rela keys init)"
ALICE_PUB="$(cat "${KEYS_WORK}/alice.pub")"
"${RELA}" keys init \
    --recipient alice \
    --pub "${ALICE_PUB}" \
    --identity "${KEYS_WORK}/alice.key" >/dev/null

if ! head -c 22 entities/requirements/REQ-002.md | grep -q '^age-encryption.org/v1'; then
    fail "REQ-002.md is not sealed"
fi
if ! head -c 22 "${REL_FILE}" | grep -q '^age-encryption.org/v1'; then
    fail "${REL_FILE} is not sealed"
fi
if grep -q "DEC-001" "${REL_FILE}"; then
    fail "${REL_FILE} still leaks endpoint IDs in cleartext after keys init"
fi
pass "entity AND relation files now start with the age header (ciphertext on disk)"

STATUS_OUT="$("${RELA}" keys status)"
grep -q "Recipients (1)" <<<"${STATUS_OUT}" || fail "expected 'Recipients (1)' in keys status"
grep -q "alice" <<<"${STATUS_OUT}" || fail "expected alice in keys status"
pass "keys status reports 1 recipient (alice)"

step "5. Alice reads; bob cannot (bob is not yet a recipient)"
ALICE_SEE="$("${RELA}" show REQ-002 2>&1)"
grep -q "Confidential" <<<"${ALICE_SEE}" || fail "alice should read REQ-002 (got: ${ALICE_SEE})"
pass "alice reads REQ-002 through CLI"

set +e
BOB_SEE="$(RELA_KEY_FILE="${KEYS_WORK}/bob.key" "${RELA}" show REQ-002 2>&1)"
BOB_EXIT=$?
set -e
[[ ${BOB_EXIT} -ne 0 ]] || fail "bob should be refused but show succeeded: ${BOB_SEE}"
pass "bob is refused (error: '$(echo "${BOB_SEE}" | head -1)')"

step "6. Add bob as a recipient"
BOB_PUB="$(cat "${KEYS_WORK}/bob.pub")"
"${RELA}" keys add bob --pub "${BOB_PUB}" >/dev/null

BOB_SEE="$(RELA_KEY_FILE="${KEYS_WORK}/bob.key" "${RELA}" show REQ-002 2>&1)"
grep -q "Confidential" <<<"${BOB_SEE}" || fail "bob should read REQ-002 after keys add"
pass "bob reads REQ-002 after being added"

set +e
EVE_SEE="$(RELA_KEY_FILE="${KEYS_WORK}/eve.key" "${RELA}" show REQ-002 2>&1)"
EVE_EXIT=$?
set -e
[[ ${EVE_EXIT} -ne 0 ]] || fail "eve should be refused but show succeeded: ${EVE_SEE}"
pass "eve is still refused (never a recipient)"

step "7. Remove bob (re-encrypt under alice only)"
"${RELA}" keys remove bob >/dev/null

# Bob's identity no longer decrypts new on-disk state.
set +e
BOB_SEE="$(RELA_KEY_FILE="${KEYS_WORK}/bob.key" "${RELA}" show REQ-002 2>&1)"
BOB_EXIT=$?
set -e
[[ ${BOB_EXIT} -ne 0 ]] || fail "bob should be refused after keys remove: ${BOB_SEE}"
pass "bob is refused after removal"

"${RELA}" show REQ-002 2>&1 | grep -q "Confidential" || fail "alice should still read REQ-002"
pass "alice still reads REQ-002"

step "8. Decrypt whole project back to cleartext"
"${RELA}" keys decrypt >/dev/null

grep -q "Confidential" entities/requirements/REQ-002.md || fail "cleartext not restored after decrypt"
if head -c 22 entities/requirements/REQ-002.md | grep -q '^age-encryption.org/v1'; then
    fail "file still sealed after decrypt"
fi
grep -q "from: DEC-001" "${REL_FILE}" || fail "cleartext relation not restored after decrypt"
if head -c 22 "${REL_FILE}" | grep -q '^age-encryption.org/v1'; then
    fail "relation still sealed after decrypt"
fi
[[ ! -f ".rela/encryption.yaml" ]] || fail "encryption.yaml still present after decrypt"
[[ ! -d "keys" ]] || fail "keys/ still present after decrypt"
pass "project is cleartext again; marker and keys/ dir removed; entity + relation content preserved"

"${RELA}" show REQ-002 2>&1 | grep -q "Confidential" || fail "REQ-002 not readable in cleartext mode"
pass "REQ-002 readable without any identity"

step "Demo complete"
printf '\n\033[32mAll invariants held across cleartext -> encrypted -> add -> remove -> decrypt.\033[0m\n'
