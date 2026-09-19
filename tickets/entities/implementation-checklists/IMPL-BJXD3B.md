---
id: IMPL-BJXD3B
type: implementation-checklist
title: 'Implementation: Faced relation history: capture skipped state-tailed edges and reads addressed the default tail'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

The tail travels with the capture and with the read, instead of being dropped:

1. `entitymanager/version_hook.go` — the `if !r.FromFace.IsDefault()` skip is
gone; `RelationVersionRecord.FromFace` carries the tail, exactly as an entity's
`VersionRecord` carries `Face`.
2. `pgstore`/`sqlitestore` `recordIDForKey` — the composite key now includes
`from_face`, in both the live-row query and the deleted-lineage fallback, so a
state-tailed capture resolves its OWN lineage.
3. `store.RelationHistoryQuery.FromFace` and a `fromFace` parameter on
`ListRelationLifetimes` — reads select a tail rather than always the default.
4. `dataentry`/`cli` — the source address is parsed, so the face reaches the
store and the bare id reaches the ACL gate.

Two incidental defects were found and fixed rather than left:

- `sqlitestore.WriteRelationVersion` never resolved a record id at all, so
every synchronous capture inserted `rel_record_id = 0` — one shared lineage for
every deleted relation. Pre-existing and independent of faces.
- `pgstore.contentHashOfRelation` omitted `FromFace`, unlike sqlitestore's and
unlike `canonical.HashRelation`, so two tails with identical bytes hashed the
same and a content-keyed dedup could drop one tail's capture.

Edge cases: purge stays default-tail (its request type names no face) and each
call site says so explicitly rather than by omission; the rename-stitch walk
already matched `rv.from_face = ren.from_face`, so scoping the lifetime heads to
one tail is consistent with it; a tail with neither a live edge nor history is
`ErrNotFound` rather than silently filed under the default lineage.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Store contract: a `RelationTails` group in `storetest/version.go`, so both
database backends are held to one contract rather than each describing its own
behaviour. Three cases: independent lineages per tail (reading each tail's
history BY FACE and asserting it gets that tail's snapshot), distinct content
hashes on byte-identical content, and `ErrNotFound` for an unknown tail. A
shared `seedTails` helper builds both tails of one triple.

HTTP level: both directions of the address parse are pinned, because a one-sided
test passes against a handler that reads whichever tail sorts first. The test
fake keys on the tail exactly as the real store's lineage resolution does — a
fake keyed on the triple alone cannot tell a faced read from a bare one and
would pass a handler that drops the face.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

The fix was verified by breaking it. With the sqlite resolution step disabled,
`RelationTails` fails exactly as the defect predicts:

```
TailsHaveIndependentLineages: "[{1 0 2 ...}]" should have 2 item(s), but has 1
  -> both captures on rel_record_id 0, one lineage holding two versions
IdenticalContentAcrossTailsStaysDistinct: same
UnknownTailIsNotFound: Expected error "store: not found" but got nil
```

That is the interleaving this ticket exists to prevent, observed directly rather
than argued from the code.

The wire format was checked end-to-end separately: the SPA's
`encodeURIComponent` produces `POL-1%40published`, which Go decodes back to
`POL-1@published` in `URL.Path` before the handler splits it — so the address
the SPA already sends reaches the new parse.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed: the entity `Faces` history suite is the precedent for
`RelationTails`, including its "identical content across faces stays distinct"
case; `parseEntityRef` is the precedent for the address parse, and its
malformed-address-is-a-404 rationale is reused rather than restated.

DRY: `currentEdgeOnFace` was lifted off `writeHandler` into a package-level
`edgeOnFace`, because relation-history restore needed the same face-aware edge
read and the store has no face-aware relation get. A second copy of that query
is how one of the two ends up addressing the wrong edge.

**Security.** The read gate is unchanged in shape and strictly better fed: it
now receives the bare id it always expected, where before a faced address gave
it a string matching no row. That failure was fail-closed for the row gate (404)
but fail-open in effect for redaction — a live relation's `meta` was served
empty to everyone, which BUG-OOZBBK records. `ListRelationLifetimes` being
scoped to one tail also narrows what a caller learns: previously it returned
`RecordID` handles for every tail of a triple, with no face in the response to
distinguish them.

Gates: `just ci` passes end to end (including the docs freshness check), plus
the sqlite- and postgres-tagged suites with `-race` against a local database. CI
on PR #1624: Test, Lint, Build, SQLite Backend, Postgres Backend, E2E, Frontend,
Docs, Architecture, Comment lint, God-object lint, CodeQL, Semgrep, Fuzz and all
cross-compiles pass.

**Scope split, filed not hidden:** the original ticket's restore half is
TKT-25T2GW (entity restore replaces the face's links) — genuinely unbuilt design
work, moved out so this ticket describes only what shipped. The unparsed-address
defect is BUG-OOZBBK, with a prevention measure, because it is the second
occurrence of that shape.
