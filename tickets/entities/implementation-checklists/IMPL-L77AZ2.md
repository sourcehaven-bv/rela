---
id: IMPL-L77AZ2
type: implementation-checklist
title: 'Implementation: Document the ctag watermark''s cross-collection activity signal as accepted residual risk'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new code. The Go diff is
provably comment-only — `git diff develop -- internal/` with comment lines
filtered out is empty. A test over comment prose would pin wording, not
behaviour, and would fail on every future rewording while asserting nothing
about the program.)
- [x] ~~Integration tests written~~ (N/A: same reason. The behaviour this
documents is already pinned by
`TestEntityTypeWatermark_DeleteDoesNotGoBackwards`,
`TestEntityTypeWatermark_ScopedByType` and
`TestEntityTypeWatermark_RelationDeleteDoesNotMoveEntityType` in
`internal/store/pgstore/watermark_test.go` — those existed before this ticket
and are exactly what the new prose describes.)
- [x] Happy path implemented — the `store.TypeWatermark` godoc gained a
`# The over-triggering is also a disclosure, and it is accepted` section, and
`docs-project/entities/guides/GUIDE-caldav.md` gained one `Constraints` bullet.
- [x] Edge cases from planning handled. Of the four the plan listed:
  - fsstore has no watermark and takes the ETag path, so the signal does not
exist there. Handled by scoping: the whole section lives on the `TypeWatermark`
interface, whose opening already says fsstore deliberately does not implement
it. Not restated — that would be padding.
  - A type with only one collection has no cross-collection signal. Handled by
the section's own framing ("only appears once one type is exposed through
several differently-authorized collections").
  - The 0-for-never-populated case is already documented on the method; not
re-litigated.
  - Static collections are operator-declared, so the finding is specifically
about DYNAMIC ones. The section names `project_tasks--PRJ-1` /
`project_tasks--PRJ-2` rather than speaking of collections generally.
- [x] ~~Error handling in place~~ (N/A: no new error paths. `watermarkCTag`'s
existing error propagation is untouched.)

## Test Quality

- [x] ~~Fixture builders or factories~~ (N/A: no tests added.)
- [x] ~~No hardcoded values in assertions~~ (N/A: no tests added.)
- [x] ~~Only specifying values that matter~~ (N/A: no tests added.)
- [x] ~~Interpolated values constructed from objects~~ (N/A: no tests added.)
- [x] ~~Property comparisons use original object~~ (N/A: no tests added.)

## Manual Verification

- [x] Feature manually tested end-to-end — for a docs change the equivalent of
an end-to-end test is verifying every factual claim against the code, because
prose has no compiler and nothing else will catch a false assertion. All were
checked at file:line; see Verification Evidence.
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified — see Development above.

**Verification Evidence:**

Being explicit about what IS and IS NOT verifiable here, per the ticket's docs
kind. Nothing below is inferred; each line names where it was checked.

*Claims verified against the code (the substantive work of this ticket):*

1. `internal/store/pgstore/watermark.go:28-40` — `EntityTypeWatermark` is
`COALESCE(MAX(seq), 0)` over `entities WHERE type = $1` UNION ALL `deletions
WHERE kind = 'e' AND typ = $1`. Confirms: the only scoping predicate is the
entity type. No ACL, no filter, no collection. The "type scope only" claim is
exact.
2. `internal/store/pgstore/tombstone.go:12-17` — `writeEntityTombstone` is
`INSERT INTO deletions (kind, id_a, typ) VALUES ('e', $1, $2)`. Id and type
only; no relations, no properties. Confirms "a narrower scope cannot be
reconstructed once the row is gone" at the SQL level, and confirms
`writeEntityTombstone` is the concrete thing that would have to change first.
**Corrected against the salvaged draft**: it called this a "two-column INSERT".
It is three columns (`kind, id_a, typ`) with two bound parameters. The draft's
phrasing was wrong on a checkable fact and is now removed rather than merely
reworded — the count carried no argumentative weight, so it was replaced with a
reference to the function itself.
3. `internal/dataentry/caldav_backend.go:655-676` — `watermarkCTag` hashes
`"wm:%d:%s:%d"` over `(len(name), name, seq)`. Confirms BOTH halves of the
finding: the collection name is mixed in (so two collections do not share a tag
VALUE, and the salvaged text's insistence on this distinction is right and worth
keeping), and `seq` is the only input that varies over time (so they move at the
same TIME). The existing comment there says the namespacing exists so the tags
do not collide — it does not claim the tags move independently, so the new text
extends that comment rather than contradicting it.
4. `internal/dataentry/caldav_backend.go:686-702` — `collectionCTag` calls
`watermarkCTag` first and falls through to per-entry ETag hashing only when `ok`
is false. Confirms bullet 3's framing: ETag hashing is the EXISTING fallback
path, not an alternative design one could newly adopt.
5. `internal/store/store.go:585-625` — the pre-existing godoc. Confirmed it
argues functional safety only ("a spurious re-sync costs one listing and
self-corrects; a missed change strands a client forever") and never uses the
words confidential, principal-as-adversary, or disclosure. The gap the ticket
describes is real, not a misreading.
6. `internal/store/pgstore/watermark_test.go:44-62` —
`TestEntityTypeWatermark_DeleteDoesNotGoBackwards`. The backwards-`max(seq)`
failure the first rejected alternative describes is not hypothetical: it is the
exact regression this existing test was written to pin, for the type scope. The
new bullet asserts the same failure would return at a narrower scope, which
follows because the tombstone lacks the narrowing key (item 2).

*Acceptance criteria:*

- AC1 (a reader at `TypeWatermark` finds the confidentiality analysis without
leaving the godoc) — met. The section answers the two-principals scenario
in-place; it links out only for the operator-facing restatement.
- AC2 (says why per-scope is unavailable, not merely unimplemented, and names
the precondition) — met, and strengthened during review. The draft named the
GDPR question but left the ordering implicit; it now states it as a sequence
("answer it, then widen the tombstone, then narrow the watermark. Not in the
other order") and names `writeEntityTombstone` so the reader can find the row.
- AC3 (CalDAV guide states it in operator terms, in `Constraints`) — met. One
bullet, alongside the existing honest caveats, ending in the actionable
deployment advice (separate types, not separate collections over one type).
- AC4 (`docs/caldav.md` generated, not hand-edited) — met. `just docs` produced
NO diff, which proves the checked-in generated file already matches what the
generator emits from the entity.
- AC5 (no behaviour change) — met, and this is the one claim here that is
mechanically provable rather than editorial: `git diff develop -- internal/`
with all comment lines filtered produces empty output. Plus `just test` green.

*What is NOT verifiable, stated plainly:* whether the prose is GOOD — clear,
correctly weighted, worth its length — is editorial and cannot be gated. That
judgement belongs to the PR review, which is where this ticket's real
verification happens. No test evidence is claimed for it.

*Gate results:* `just arch-lint` OK, `just plimsoll` clean, `just comment-lint`
clean (no unresolvable doc links across 11461 comments — relevant because the
new text deliberately references `writeEntityTombstone` WITHOUT brackets, since
Go cannot link an unexported symbol in another package), `just lint-md` 0 issues
in 257 files, `just docs` no diff, `just test` green, `rela validate` clean.

## Quality

- [x] Code follows project patterns — the new godoc section uses the same
`# Heading` structure as the four sections above it, and is deliberately shaped
like the sibling `# What this does NOT cover` section on `watermarkCTag`: an
admitted limitation, its consequence, and the condition under which it would be
fixed. The CalDAV bullet matches the surrounding `Constraints` entries, which
are all honest caveats with a workaround.
- [x] Checked for DRY opportunities. The relevant risk here is `commentlint`'s
`duplication` rule, since the new text sits two sections below arguments it
touches. Two facts were deliberately CITED rather than restated: the quadratic
ETag cost (bullet 3 points at `# Why this exists` instead of repeating the
P-renders-per-poll argument) and the backwards-tag failure (bullet 1 points at
"the section above" instead of re-deriving it). Both citations are load-bearing
in their new context — the same failure applied to a different scope — which is
why they are one clause each rather than a paragraph.
- [x] No security issues introduced. This documents a security finding rather
than introducing one; nothing is weakened. The compensating controls the
analysis relies on were confirmed present and untouched: the ACL gate on driver
lookup (`resolveDynamic` → `getEntity` visibility) and the uniform 404 for a
well-formed dynamic name whose driver is absent OR unreadable
(`caldav_backend.go:165-168`, `296-299` — the comment there explicitly makes
absent and unreadable indistinguishable). The text was also written to describe
the signal's SHAPE and BOUND rather than to read as a procedure for exploiting
it.
- [x] ~~No silent failures~~ (N/A: no error paths added or changed.)
- [x] No debug code left behind — diff is comments, one docs entity, its
generated output, and ticket entities.
