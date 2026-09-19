---
id: IMPL-F6NQTW
type: implementation-checklist
title: 'Implementation: Relation-history route never parsed the source address'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`handleV1RelationHistory` parses `parts[1]` with `parseEntityRef` instead of
using it verbatim. The bare id goes to the ACL gate (which is face-blind by
design), the face selects the tail. Both downstream consumers are fixed by the
one change: `authorizeRelationHistoryRead`'s endpoint lookups and
`serveRelationHistoryVersion`'s live-source lookup for redaction.

The restore branch on the same route was addressing the default tail too: it now
carries `RelationOptions.FromFace` and probes liveness with `edgeOnFace`, so a
faced restore updates the edge the caller addressed instead of creating a second
one beside it.

Responses echo the address the caller used (`fromRef.String()`), not the bare
id, so a client cannot be told it read a different edge than it asked for.

Edge cases: a malformed address is the uniform not-found rather than a 400,
matching `parseEntityRef`'s existing rationale — a syntactically impossible
address cannot name a row, and a distinct error would only tell a caller which
strings are worth probing.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`TestRelationHistory_FacedAddressReadsItsOwnTail` and
`TestRelationHistory_BareAddressReadsTheDefaultTail` — both directions, since a
one-sided test passes against a handler that reads whichever tail sorts first.
Each seeds two tails with distinguishable bodies and asserts on the body
returned, so the assertion sees which edge was actually read rather than only
that the route answered.

The fake's key changed from the triple to `FormatStateRef(from, face)|type|to`,
mirroring the real store's lineage resolution. A fake keyed on the triple alone
cannot distinguish a faced read from a bare one, so it would have passed the
broken handler — which is precisely why the existing tests did.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

The wire path was confirmed rather than assumed. The SPA builds the URL with
`encodeURIComponent`, giving `POL-1%40published`; a standalone check confirmed
Go decodes that back to `POL-1@published` in `URL.Path` before the handler
splits it. So the faced address the SPA already sends reaches the new parse — no
SPA change was needed, and none was made.

Backend sensitivity was verified by reading the lookup contracts: pgstore
resolves by `(id, face)` columns so a suffixed id matches nothing, while
fsstore/memstore key their index on `FormatStateRef` so it accidentally
resolves. That asymmetry is why the defect never showed on the default build.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed: `parseEntityRef` is what every other faced-aware route uses;
this route simply joins them.

DRY: the face-aware edge read is now the shared `edgeOnFace` rather than a
second copy of the `RelationQuery` shape.

**Security.** This fixes a redaction failure, so it is the load-bearing part.
The live-source lookup missing meant `serveRelationHistoryVersion` took its
"deleted source" branch and served empty `meta` for a live relation — its
properties withheld from every caller, including those entitled to them. The row
gate itself was fail-closed (a 404), so no data was exposed; the defect was
over-withholding plus an unusable route, not disclosure. Dual-endpoint gating
(FROM ∧ TO) is unchanged, and the gate now receives the bare id it always
expected.

Gates: full `just ci`, plus the sqlite- and postgres-tagged suites with `-race`.
CI on PR #1624 green across Test, Lint, Build, both backend suites, E2E,
Frontend, Docs and the static-analysis jobs.
