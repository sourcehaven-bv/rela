---
id: RR-DC30OF
type: review-response
title: The editor's ID validator was a stale denylist that accepted ids the store rejects
finding: 'isValidEntityRefId carried a hand-rolled denylist (no --, no control chars, no / \ backtick space) whose doc comment cited insertEntityRef.ts, a file this change deletes. The real backend grammar is an ALLOWLIST — entity.ValidateID uses ^[A-Za-z0-9][A-Za-z0-9_-]*$ plus no -- and no .. — since storeutil.ValidateID was collapsed into it (TKT-IZGF7T). The editor therefore accepted ids the store refuses: `a.b` and `iso-27001-a.5.1` rendered as references to entities that cannot exist. An existing test asserted the store accepted `iso-27001-a.5.1`; running entity.ValidateID against it shows it does not.'
severity: significant
resolution: 'Replaced the denylist with the real grammar. Added a shared JSON fixture (26 cases) read by BOTH a vitest suite and a new Go test against entity.ValidateID, so the two cannot drift silently. Verified non-vacuous: marking `a.b` valid fails on both sides.'
status: addressed
---

## Finding

Three problems stacked:

1. The doc comment pointed at `insertEntityRef.ts`, which this change deletes.
2. The rule was a denylist; the backend's is an allowlist. A denylist that has
fallen behind fails *open*.
3. A test asserted `iso-27001-a.5.1` is accepted by the store. Running
`entity.ValidateID` against it returns `invalid characters in entity ID`.

So the editor would render `` `a.b` `` as a link to an entity that cannot exist.
Not a security problem — the title still comes from the mentions map, which
would have no entry — but a plainly wrong affordance.

## Root cause

`storeutil.ValidateID ` was collapsed into `entity.ValidateID ` under
TKT-IZGF7T, tightening the store rule to one grammar. The frontend copy mirrored
the pre-collapse version and nothing connected them, so the tightening never
propagated. Exactly the failure mode the "kept in step with that file" comment
claimed to prevent while providing no mechanism.

## Resolution

`ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9_-]*$/ ` plus the `-- ` and `.. `
checks, matching the Go rule.

The mechanism that matters is the shared fixture,
`testdata/entity-id-grammar.json `, read by both `entityRefIdGrammar.test.ts `
and `internal/entity/id_grammar_fixture_test.go `. Both assert every case, so a
change to either grammar without the other fails a test. Each side also guards
against an emptied fixture, since a passing run over zero cases proves nothing.

Verified by adding `a.b ` to the valid list: both languages fail, then pass on
restore.
