---
id: IMPL-H3JT9B
type: implementation-checklist
title: 'Implementation: A denied ?world= short-circuits past worldCapablePath, serving default-world content on refused routes'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

The regression test drives the REAL router with a real `acl.Declarative`, so it
exercises middleware ordering, the grant check and the handler together — the
unit-level `refuseWorldIncapablePath` test only pins the default-world boundary
the integration test cannot reach cheaply.

Error handling is untouched by this change: the `err != nil` arm still surfaces
an infrastructure failure as a 500 rather than folding it into a denial
(RR-4TFZNL), and the new refusal sits ahead of it without swallowing anything.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values in these assertions)
- [x] Property comparisons use original object, not hardcoded strings

`appForWorldGrant(t, granted bool)` is the factory — one seam, one boolean, so a
reader sees immediately that the grant is the ONLY difference between the two
responses being compared. That matters more than usual here: the test's whole
claim is that nothing else varies.

Assertions compare the two recorders against EACH OTHER rather than against
expected literals, which is what makes the test durable — it keeps failing for
any future divergence, not just the one that motivated it. The single literal
(`secretdraft`) is the seeded title, and it is asserted absent, not equal.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

*The tests genuinely catch the bug.* Disabled only the new call (`if false &&
refuseWorldIncapablePath(...)`), keeping the helper defined so the package still
compiled, and re-ran: all five subtests failed, including the explicit `LEAK: a
denied world was served DEFAULT-world content` assertion on `_analyze`.
Restored, all pass.

*Precedence.* Confirmed by hand that `?world=bogus` and `?world=a&world=b`
return their 400s identically on `_analyze` and `/tickets`. An earlier placement
of the check shadowed both with the 422 on non-capable routes — caught during
self-review, moved below the two 400 arms, and pinned by
`TestRefuseWorldIncapablePath_DoesNotShadowConfigErrors`.

*Nothing over-refused.* `/api/v1/tickets?world=published` with a denied grant
still returns 200 with an empty list and its real `meta`/`_actions`, so the
empty-result design that closes the existence oracle is intact.

*Full checks.* `go test ./internal/dataentry/` (31s, ok), `just arch-lint`,
`just comment-lint` (gate, 0 unresolvable doclinks), `just plimsoll`, `just
coverage-check`, `golangci-lint run ./internal/dataentry/...` (0 issues) — all
pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

`refuseWorldIncapablePath` is a free function taking `w`/`r`, matching the
file's stated convention for `resolveWorld` and `configuredDefaultWorld` — `App`
sits at its plimsoll load line, and adding a method is the habit that got it
there.

On DRY: the extraction removed a genuine duplication risk rather than creating
an abstraction for its own sake. The refusal now has exactly one call site, but
it needed a name to carry the ordering rationale, which is the load-bearing
part. Two historical notes (`?include=`, `?q=`) moved with it, since they warn
against restoring refusals at what is now this function's position — leaving
them behind would have stranded them pointing at deleted code.

Security: the change only ever NARROWS. A route that previously served a denied
world now refuses it; no route gained access it lacked.
