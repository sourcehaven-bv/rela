---
id: IMPL-UMIBIP
type: implementation-checklist
title: 'Implementation: Scope the timing claim in the ACL guide to entity-level filtering'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Documentation only. No code changed.

The original sentence is KEPT — it is correct about rows — and the correction
scopes it rather than replacing it. Deleting it would have lost a real property
(pgstore pushes entity-level filtering into the query, so a hidden row costs
nothing observable) in order to avoid stating a limit.

Three things added:

- **The distinction.** Entity-level filtering is in the query; property-level
`visible:` redaction runs in Go over already-fetched rows, so it is not
constant-time.
- **Why that trade is right.** `visible:` grants are per-principal and can be
conditional on graph predicates, so pushing them into the store means
reimplementing the affordance resolver in SQL for pgstore and again for fsstore
and memstore — a permanent, backend-multiplied correctness burden on the most
security-sensitive code in the tree, against a signal measured in microseconds
by an attacker who must already hold a valid principal and already know which
field to probe.
- **Why the residual signal is a consequence of doing the hard part.** It exists
BECAUSE rela does field-level redaction at all. Most applications have no
central point where entity access is decided and no field-level visibility to
speak of — a "hidden" field is one the template happens not to render.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

N/A — no tests. A timing test was considered and rejected in planning: it would
be inherently flaky at microsecond scale on a shared runner, and would pin the
CURRENT performance characteristics as a requirement, so any optimisation to the
redaction loop would redden it. Measuring the signal is not the same as
defending against it, and the decision is that it does not need defending.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

The correction claims rela has a central enforcement point for entity access.
That is the premise the "strength, not shortcoming" framing rests on, so it was
checked rather than asserted:

| claim | how checked | result |
| --- | --- | --- |
| gating is structural, not by convention | read `internal/dataentry/visiblereader.go` | its own doc says the type "holds the store privately and exposes only gated reads" precisely because gate-by-convention was the read-ACL bug class (TKT-N26KLB, #1010) |
| no HTTP read path returns unredacted properties | `grep -rn "a.store.GetEntity" internal/dataentry/*.go` (non-test) | one hit, `api_v1.go:2326` |
| that one hit is not a leak | read the surrounding handler | reads the entity only to compare its TYPE for a document-kind mismatch, returns no properties to the caller, and an ACL gate follows immediately |

Had the second row returned several ungated reads, the framing would have been
wrong and this ticket would have become a different one.

Gates: `just lint-md` 0 issues; `just docs` regenerated cleanly, so the
generated `docs/acl-security.md` matches its `docs-project/` source (the Docs CI
check compares them).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

DRY: N/A.

Security: the security-relevant act here is stating a limit ACCURATELY, in both
directions. An overclaiming security doc is worse than a silent one — a reader
who trusts "no timing signal" may build on it, and the correction is cheap now
and expensive after someone has. But underclaiming would be its own failure:
describing this as a vulnerability would misdirect effort toward a
microsecond-scale signal and misrepresent a system that does more field-level
enforcement than most.

The wording was chosen for that balance: "not constant-time" is precise;
"vulnerable to timing attacks" would not be.
