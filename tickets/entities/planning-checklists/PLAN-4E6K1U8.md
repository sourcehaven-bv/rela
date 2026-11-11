---
id: PLAN-4E6K1U8
type: planning-checklist
title: 'Planning: Two-way sync with a remote: push and held conflicts (9a)'
started: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope (owner answers 2026-11-11, DEC-2NFLAHB):** 9a is the id mapping in the
driver's ledger, push (POST, and PATCH with If-Match plus one retry on 412),
the hold rule, the conflict list and its CLI, share scope per type, and the
e2e echo test. Out: the latest, local and remote rules and pushing relations
(TKT-8ZGAEC5, 9b); deletes (TKT-CVT9W1K). Ids follow the API rules: a pushed
row gets a fresh id on the remote, and only the driver keeps the local id to
remote id mapping.

**Acceptance Criteria:**
1. Two instances (a real dataentry API on the remote, httptest): a local
edit and a new local row reach the remote; a remote edit reaches the local
side; a field both sides changed is held and written on neither side.
2. Echo stop: after a sync, two more syncs with no edits leave every entity
version and the audit log unchanged on both sides.
3. A 412 is merged and retried once; a second 412 waits; a 403, 409 or 422
holds the row on the conflict list and is never overwritten.
4. `rela remote conflicts` lists held entries; `rela remote resolve` resolves
one and the sync it runs sends the winner.
5. A remotes.yaml that names latest, local or remote fails to load with a
clear message; share needs push and refuses related(...).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: covered by RES-G8CYB1R)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-G8CYB1R

**Existing Solutions:** step 8 `internal/remotesync` (ledger with field digests,
three-way merge in `merge`), `httpremote` (GET only before), the PATCH
If-Match support of the data-entry API, `predicatefns` for share.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- remotes.yaml: `push: true`; `conflict:` per remote, type and field (hold
only); `share:` per type, a predicate on the local row.
- Ledger: per linked row the remote ETag. A pushed create adds a row that maps
the local id to the remote id; the local `external_ref` is set to the remote id
so a lost ledger relinks instead of creating twice.
- Run: pull as step 8, but merge continues past a conflict (other fields still
merge) and holds the field on the conflict list; then push changed fields of
linked rows by PATCH with If-Match, and new shared rows by POST without an id.
A pushed value becomes the field's base (echo stop).
- Conflict list: one state.KV document per remote, pruned with the ledger.
Resolve moves the field base to the losing side, then runs one sync under the
run lock.
- httpremote gains `Send` (GET, POST, PATCH) returning status, body and ETag.

**Alternatives rejected:** same ids on both sides (owner: follow the API id
rules); recording remote hashes with a `_reconcile` call after the push (the
next diff fetches pushed rows once and finds every field at its base, which is
simpler and needs no extra round trip).

**Dependencies:** remotesync gains predicate and predicatefns (arch-lint).
No change to entitymanager, dataentry or apiwire.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** remotes.yaml (trusted, operator); remote answers
(untrusted, as step 8): a create must answer with a new row of the requested
type and a valid id the ledger does not map; refusal details are sanitized and
capped; ETags are capped and must hold no CR or LF; methods are an allowlist.

**Security-Sensitive Operations:** writes on the remote as the same principal
and credential as the pull (no new credential, DEC-QKLMMD4); since
RR-XLXQ0XR the push reads local rows through `Deps.Gate`, so rows the run's
principal cannot read and fields hidden by `visible:` are not sent, and
`share` and `fields` narrow what leaves the project further; the conflict list holds remote values outside the graph (step 10
must gate display by entity and field read). Credentials are never logged.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1, AC2 and the resolve part of AC4:
`internal/dataentry/remote_push_e2e_test.go`. AC3:
`TestPush_RetriesOnceAfterPreconditionFailed`, `TestPush_RefusalHoldsTheRow`.
AC4: `TestPush_ConflictIsHeldAndResolved`, scheduler and CLI tests. AC5:
`TestParse_Errors`.

**Edge Cases:** cleared field (unset on PATCH, left out of POST); lost ledger
after a create; linked row leaving share scope; refused create with no remote
value (resolves to local only); local row deleted while held; dry run.

**Negative Tests:** 5xx fails the row, 404 defers it, 401 fails the run,
oversize answer, bad If-Match, unknown method.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** a first run with push creates every unref'd shared row on the remote
(documented; dry run shows the count); the push reads all rows of the mapped
types per run (bounded by max_rows; fetch budget counts push requests).
Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/remote-sync.md (two-way sync, share, conflicts
and their CLI), docs/acl-security.md via GUIDE-acl-security (push threat model).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-DI92SQY (significant, addressed: an uncertain
create is held, not resent), RR-ZUVW8WM (minor, addressed in docs: rows
mirrored from another remote), RR-IS39JPA (minor, addressed: share errors are
reported), RR-XGCZSM5 (minor, deferred: conflict list size).
