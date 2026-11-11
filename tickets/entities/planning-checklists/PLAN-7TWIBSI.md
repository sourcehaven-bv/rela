---
id: PLAN-7TWIBSI
type: planning-checklist
title: 'Planning: Two-way sync: latest/local/remote rules and relation push (9b)'
started: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope (owner answers 2026-11-11):** the conflict rules latest, local and
remote per remote, type and field; `max_clock_skew` per remote (default 5m);
`_updated` on per-entity and single-relation reads when nothing on the row is
hidden; the losing local value of `latest` kept on the conflict list (capped at
16 KiB) and `rela remote resolve --dismiss`; If-Match and 412 on relation
PATCH; push of relations between linked rows with a three-way merge of meta.
Out: deletes (TKT-CVT9W1K), approval, status page and conflict UI (TKT-0FMYPEZ),
relation content push (the API cannot write it).

**Acceptance Criteria:**
1. Each rule (hold, latest, local, remote) works at remote, type and field
level, in unit tests and in the two-instance test.
2. A row with a field hidden from the reader gets no `_updated`; a `latest`
conflict on it is held with reason `time_withheld`.
3. A remote time beyond now + max_clock_skew, or before the last sync, holds
the field with a reason; equal times hold.
4. A `latest` conflict keeps the losing value on the list as superseded; it
does not block, and `rela remote resolve --dismiss` removes it.
5. Relation PATCH honours If-Match (412 on mismatch) and answers an ETag.
6. A relation created locally between linked rows reaches the remote; meta
edits merge both ways with the rules; a relation whose type has `visible:`
meta grants, or with an unlinked endpoint, is skipped with an issue.
7. Fixes #2191, #2192, #2193 (godoc) and #2194 (PLAN-4E6K1U8 text).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: covered by RES-G8CYB1R)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: extends the in-house step 8 and 9a engine)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: covered by RES-G8CYB1R)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-G8CYB1R

**Existing Solutions:** 9a `merge` and the conflict list
(`internal/remotesync`), entity PATCH If-Match (`write_handler.go`),
`store.VersionOf` and `ExpectedVersion` CAS, `attachEntityAffordances` for the
per-entity `_redacted` decision, `RelationMetaGated` in affordances.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- Config: accept the rules; precedence field > type > remote, resolved into
each field mapping at compile time, kept out of the mapping hash. Relation
mappings gain `conflict:` and `field_conflicts:`. `max_clock_skew` per remote.
- Merge: `remote` (or latest won by the remote) takes theirs; `local` (or
latest won locally) keeps ours. Both move the base to the remote digest, so the
push sends ours and the next pull agrees. `latest` holds with a reason when a
time is withheld, unparsable, beyond now + skew, before the last sync, or equal.
- API: `_updated` on `v1.Activity`, set by `attachEntityAffordances` only when
`_redacted` and the inaccessible list are empty; on the relation GET only when
`_redacted` is empty. Relation PATCH reads If-Match and pins
`RelationOptions.ExpectedVersion`, checked by the manager inside the store Tx.
- Relation push: after the entity push, list local relations of each mapped
type through the read gate, push those with both endpoints linked (POST
create, PATCH with If-Match and one retry). Pull on push remotes merges
relation meta three-way like entities.
- Conflict list: reason `superseded` for latest losers, kept under a separate
key so it never blocks; relation entries carry `kind: relation`.

**Alternatives rejected:** comparing ETags instead of times for latest (does
not order edits); serving `_updated` always (a hidden field's edit time is a
disclosure); relation meta source-wins on push remotes (loses local edits).

**Files to modify:** internal/remotesync (config, run, push, conflicts,
source, report, new relpush), internal/scheduler, internal/cli
(remote_conflicts), internal/dataentry (affordances, relation handlers),
internal/apiwire/v1, internal/entity (writeapi), internal/entitymanager
(UpdateRelation), internal/store (version), internal/visibility, OpenAPI,
docs/remote-sync.md, docs/acl-security.md, API reference.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** remote `_updated` is untrusted: parsed as RFC
3339, bounded by now + skew and the last sync, else held. Rule names are an
allowlist. Relation answers are validated as in step 8.

**Security-Sensitive Operations:** `_updated` is a new read-path field, served
only when the reader sees every field of the row (a hidden field's edit time
would leak when it changed). Relation push reads through the read gate and
skips types with gated meta, because relation meta redaction exists only in
dataentry (TKT-0RBFN0). The superseded local value is stored outside the graph;
it is removed with the row or the remote. A hostile remote can win every
`latest` conflict within the skew window; docs say untrusted remotes should use
hold or local.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1 and AC6: remotesync unit tests per rule and level,
`TestRemotePush_EndToEnd` extended. AC2: dataentry test on `_updated` with a
hidden field, remotesync test on the fallback. AC3: clock tests. AC4: CLI and
scheduler tests for `--dismiss`. AC5: relation PATCH If-Match tests.

**Edge Cases:** cleared values (null on PATCH), equal times, missing ledger
sync time on rows from 9a, oversize local value, relation with a faced tail,
both sides created the same relation.

**Negative Tests:** unknown rule, bad skew, take on a superseded entry,
dismiss on a held entry, If-Match mismatch, relation refusal held.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** fsstore times come from file mtimes, which git checkout resets
(documented; latest then holds or picks by the reset time). Plimsoll caps on
`runner` and `v1.Entity` (new type `relPusher`, field on `Activity`).
Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/remote-sync.md (rules, skew, dismiss, relation
push, purge statement), docs/acl-security.md (`_updated`, clock trust, relation
push, relation PATCH If-Match), API reference and OpenAPI.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-ZQ54YZ5 (significant, addressed: one
superseded entry per row and field), RR-CO25745 (significant, addressed:
remote-hidden relation meta is left out), RR-3H4LV0N (minor, documented: times
are per row), RR-9VJFG26 (minor, addressed: no sync time skips the check),
RR-10P538W (minor, addressed: adoption on push remotes merges meta).
