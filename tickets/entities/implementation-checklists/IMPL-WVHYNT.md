---
id: IMPL-WVHYNT
type: implementation-checklist
title: 'Implementation: Document the help endpoint as public by design'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Documentation only — a godoc on `handleEntityHelp`. No behaviour changed.

The test boxes are checked with no tests added, which needs justifying rather
than waving through: there is no behaviour to test, and a test asserting "help
is reachable without authorization" would pin the CURRENT design as a
REQUIREMENT. That would make the decision harder to revisit than the code
documenting it — and the comment explicitly says the decision should be
re-opened if `_schema` is ever gated. A test would fight that.

The comment records three things a reader cannot infer from the code:

- **Why it is ungated** — the open-source test. The entity model, field
descriptions and help prose all live in the repository, so guarding the endpoint
that serves them protects nothing an interested party cannot read on GitHub.
Help is documentation ABOUT the application, not data IN it.
- **The already-public sibling.** `handleV1Schema` serves every type name,
property and relation ungated, from the same router. Named so a reader can CHECK
the claim rather than take it on trust.
- **What would change the answer** — if `_schema` were ever read-gated, the
argument loses its second leg and should be re-decided rather than inherited.

It also names the distinction the finding conflated: this endpoint discloses the
entity TYPE model, never an entity instance, a property value, or anything
ACL-governed. Those come from the data endpoints, which are gated. Read
authorization exists for the latter.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

N/A — no tests added, for the reason above.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

The entire argument rests on one factual claim, so it was checked rather than
assumed. If `_schema` turned out to be gated, the decision would be WRONG and a
comment asserting it confidently would be actively misleading.

| claim | how checked | result |
| --- | --- | --- |
| `handleV1Schema` performs no read gate | read the handler body (`internal/dataentry/api_v1.go:1158+`) for `readGate` / `Permits` / `Authoriz` calls | none present |
| it serves the full model | read the handler: builds `v1.Schema{Entities, Relations, Types}` from `a.State()` | confirmed — every type, property and relation |
| both are in the same router | `api_v1.go:84` (`_schema`) and `router.go:87` (`/api/help/`) | confirmed |

The third row matters more than it looks: it rules out the objection that these
are differently-secured surfaces and therefore not comparable.

Gates: `just lint` 0 issues, `just comment-lint` no unresolvable doc links
across 11461 comments, `just plimsoll` clean, `go test ./internal/dataentry/`
ok.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

DRY: N/A — a comment.

Security: no code changed. The security value is that the next reviewer
evaluates the REASONING rather than rediscovering the absence — which is what
happened here, and why the issue arrived from outside instead of being answered
by the code.

Follows CLAUDE.md's rule that a settled decision should read as SETTLED rather
than open. The code previously neither gated nor explained, and that silence is
precisely what invited the finding. Worth noting the counter-intuitive part: a
gate here would have been WORSE than the current state, not merely unnecessary,
because a boundary that looks deliberate while defending nothing invites the
reader to assume the model is private.
