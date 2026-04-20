#!/usr/bin/env bash
# End-to-end demo for the rela at-rest encryption feature.
#
# Walks through the full lifecycle:
#   1. Create a cleartext rela project with sample requirements.
#   2. Attach a file to a requirement (exercises the attachment store).
#   3. Generate age identities for alice, bob, eve.
#   4. Encrypt the project for alice only; verify attachment is now sealed.
#   5. Attach a second file under encryption; verify it lands sealed.
#   6. Verify bob cannot read; alice can.
#   7. Add bob as a recipient; verify bob can now read attachments too.
#   8. Remove bob; verify bob has lost access.
#   9. Eve (never a recipient) never has access.
#  10. Decrypt the project back to cleartext; verify attachments preserved.
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
# Add a file-typed property to the requirement schema so we can
# exercise the attachment path (C1 regression: attachments must be
# routed through cryptofs on encrypted repos, not the raw FS).
python3 - <<'PY'
with open('metamodel.yaml') as f:
    s = f.read()
s = s.replace(
    '''      priority:
        type: priority''',
    '''      priority:
        type: priority
      design_doc:
        type: file''', 1)
with open('metamodel.yaml', 'w') as f:
    f.write(s)
PY

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

# Attach a file BEFORE encryption is turned on — lands cleartext in
# the content-addressable attachment store.
CLEAR_ATTACH_SRC="${WORK}/design-pre.md"
printf 'CLEARTEXT-PRE-ENCRYPTION-DESIGN-DOC\ncontains leaky body\n' > "${CLEAR_ATTACH_SRC}"
"${RELA}" attach REQ-001 "${CLEAR_ATTACH_SRC}" --property design_doc >/dev/null

# Capture the resulting attachment path from the entity's property.
PRE_ATTACH_PATH="$(grep -oE 'attachments/[a-f0-9]{2}/[a-f0-9]+\.[a-z]+' entities/requirements/REQ-001.md | head -1)"
[[ -n "${PRE_ATTACH_PATH}" ]] || fail "pre-encryption attachment path not recorded in REQ-001"
[[ -s "${PRE_ATTACH_PATH}" ]] || fail "pre-encryption attachment file missing on disk"
grep -q "CLEARTEXT-PRE-ENCRYPTION-DESIGN-DOC" "${PRE_ATTACH_PATH}" \
    || fail "pre-encryption attachment is not the cleartext we wrote"
pass "attached a cleartext file to REQ-001 (${PRE_ATTACH_PATH##*/})"

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
"${RELA}" keys init \
    --recipient alice \
    --pub-file "${KEYS_WORK}/alice.pub" \
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

# The private key must be gitignored so the user can't commit it by accident.
if ! grep -q '^\.rela/key$' .gitignore 2>/dev/null; then
    fail ".rela/key not added to .gitignore by keys init"
fi
pass ".rela/key is explicitly gitignored"

STATUS_OUT="$("${RELA}" keys status)"
grep -q "Recipients (1)" <<<"${STATUS_OUT}" || fail "expected 'Recipients (1)' in keys status"
grep -q "alice" <<<"${STATUS_OUT}" || fail "expected alice in keys status"
pass "keys status reports 1 recipient (alice)"

# The authoritative recipient list must land at <root>/recipients.age.
[[ -s recipients.age ]] || fail "recipients.age missing after keys init"
head -c 22 recipients.age | grep -q '^age-encryption.org/v1' \
    || fail "recipients.age is not itself sealed"
pass "recipients.age present and sealed"

# Existing (pre-encryption) attachment must now be sealed too.
if ! head -c 22 "${PRE_ATTACH_PATH}" | grep -q '^age-encryption.org/v1'; then
    fail "pre-existing attachment ${PRE_ATTACH_PATH} not sealed after keys init"
fi
if grep -q "CLEARTEXT-PRE-ENCRYPTION-DESIGN-DOC" "${PRE_ATTACH_PATH}"; then
    fail "attachment plaintext still visible on disk after keys init"
fi
# The sidecar YAML (content type, original filename, added_by) must
# also be sealed — finding S4.
SIDECAR="${PRE_ATTACH_PATH}.yaml"
if [[ -f "${SIDECAR}" ]]; then
    if ! head -c 22 "${SIDECAR}" | grep -q '^age-encryption.org/v1'; then
        fail "attachment sidecar ${SIDECAR} not sealed after keys init"
    fi
    pass "pre-existing attachment + sidecar YAML are sealed"
else
    pass "pre-existing attachment is sealed"
fi

step "4b. Attach a second file under encryption (exercises C1 fix end-to-end)"
ENC_ATTACH_SRC="${WORK}/design-post.md"
printf 'SECRET-POST-ENCRYPTION-DESIGN-DOC\nmust-never-land-cleartext\n' > "${ENC_ATTACH_SRC}"
"${RELA}" attach REQ-002 "${ENC_ATTACH_SRC}" --property design_doc >/dev/null

POST_ATTACH_PATH="$(grep -oE 'attachments/[a-f0-9]{2}/[a-f0-9]+\.[a-z]+' \
    <(RELA_KEY_FILE="${KEYS_WORK}/alice.key" "${RELA}" show REQ-002 2>&1) | head -1)"
[[ -n "${POST_ATTACH_PATH}" ]] || fail "post-encryption attachment path not recorded in REQ-002"
[[ -s "${POST_ATTACH_PATH}" ]] || fail "post-encryption attachment file missing"

if ! head -c 22 "${POST_ATTACH_PATH}" | grep -q '^age-encryption.org/v1'; then
    fail "post-encryption attachment ${POST_ATTACH_PATH} landed cleartext — C1 regressed"
fi
if grep -q "SECRET-POST-ENCRYPTION-DESIGN-DOC" "${POST_ATTACH_PATH}"; then
    fail "post-encryption attachment plaintext leaked on disk — C1 regressed"
fi
pass "new attachment written under encryption is sealed (${POST_ATTACH_PATH##*/})"

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
"${RELA}" keys add bob --pub-file "${KEYS_WORK}/bob.pub" >/dev/null

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
[[ ! -f "recipients.age" ]] || fail "recipients.age still present after decrypt"
pass "project is cleartext again; recipients.age removed; entity + relation content preserved"

# Both attachments must be cleartext again, with their original bodies.
grep -q "CLEARTEXT-PRE-ENCRYPTION-DESIGN-DOC" "${PRE_ATTACH_PATH}" \
    || fail "pre-encryption attachment body not restored after decrypt"
if head -c 22 "${PRE_ATTACH_PATH}" | grep -q '^age-encryption.org/v1'; then
    fail "pre-encryption attachment still sealed after decrypt"
fi
grep -q "SECRET-POST-ENCRYPTION-DESIGN-DOC" "${POST_ATTACH_PATH}" \
    || fail "post-encryption attachment body not restored after decrypt"
if head -c 22 "${POST_ATTACH_PATH}" | grep -q '^age-encryption.org/v1'; then
    fail "post-encryption attachment still sealed after decrypt"
fi
pass "both attachments restored to cleartext with original bodies intact"

"${RELA}" show REQ-002 2>&1 | grep -q "Confidential" || fail "REQ-002 not readable in cleartext mode"
pass "REQ-002 readable without any identity"

step "Demo complete"
printf '\n\033[32mAll invariants held across cleartext -> encrypted -> add -> remove -> decrypt.\033[0m\n'
