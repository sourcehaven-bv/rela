---
id: IMPL-F33ELN
type: implementation-checklist
title: 'Implementation: Warn when unmatched_principal reject has no JWT gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

One function and one call:

- `warnUnmatchedRejectWithoutJWTGate(p *acl.Policy, jwtWired bool)` in
`internal/dataentry/router.go`.
- Called from `NewRouter` immediately before `attachACLRequest`, passing the
SAME `a.jwtGate != nil` expression that `attachACLRequest` receives.

Reading the identical expression is deliberate. A check that computed wiring
state differently could disagree with the thing it claims to describe, and the
warning would then be wrong in exactly the situation it exists for.

It takes the policy and a bool rather than the `*App`, so it is pure and
testable without constructing a server. It reads the mode through the existing
`EffectiveUnmatchedPrincipal()` so the empty default resolves the same way it
does everywhere else, rather than re-deriving that rule.

Edge cases from planning, all covered: nil policy; empty-string mode; the other
two modes. No error handling to add — this reports a condition, it does not
create one, and it cannot fail.

### What review changed (RR-0CJ47L and friends)

The first version checked ONE of the three conditions `reject` actually needs.
It fires only when the mode is reject AND a JWT gate is wired AND the
`principal_property` lookup is enabled (`router.go:451`). Checking only the gate
meant a reject policy with no `user_entity_type` / `principal_property` was
equally inert and warned about nothing — a second silent-inert state, uncovered,
in exactly the shape of the bug this ticket exists to close.

My defense was "LoadPolicy refuses that combination". It does, but
`acl.NewDeclarative` never calls `Validate`, so any path skipping `LoadPolicy`
reaches that state — including this ticket's own test, which builds a
`Declarative` directly.

Fixed by extracting `acl.Policy.RejectEffective(jwtWired bool)`: one predicate
covering all three conditions, so the warning and the thing it describes cannot
drift. The message now names WHICH condition is missing — "reject does nothing"
is not actionable without saying what to change.

Three further findings, all fixed:

- **No test pinned the ARGUMENT `NewRouter` passes** (RR-U5LKGQ). Swapping
`a.jwtGate != nil` for a literal `false` left the suite green: the composition
test asserted only that some warning appeared. Added the paired gate-wired case.
- **The `d.Policy()` call widened a typed-nil's blast radius** (RR-I1RTSU). A
typed-nil `*acl.Declarative` is not nil, and `NewRouter` would now panic at
construction rather than failing later per request. `Policy()` and the Policy
predicates are nil-receiver safe, and a test pins it.
- **The slog capture buffer was unsynchronized** (RR-FZ1CBE), against a
documented precedent in `appbuild` that already solved it.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`captureWarnings(t)` factors out the slog redirect + `t.Cleanup` restore, using
the pattern already in `reserved_principal_test.go` rather than a new one. Modes
are a table so adding a fourth mode later is one line.

The `NewRouter` test builds its `Declarative` directly instead of via
`mustNewACL`, because `reject` requires `user_entity_type` +
`principal_property`, and enabling that lookup in turn requires a
`PrincipalLookup` that `mustNewACL` does not supply. The comment says so, since
"why not the helper" is the first question a reader will have.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| mutation | expected | observed |
| --- | --- | --- |
| guard returns early always (never warn) | the positive tests redden | `..._WarnsWhenNoJWTGateWired` and `..._NewRouterWarnsWhenGateMissing` FAIL |
| drop the `jwtWired` term (warn whenever reject is set) | the false-alarm test reddens | `..._SilentWhenJWTGateWired` FAILs |
| delete the call from `NewRouter` | only the composition test reddens | `..._NewRouterWarnsWhenGateMissing` FAILs, alone |
| drop the lookup term from `RejectEffective` | the lookup test reddens alone | `..._WarnsWhenLookupDisabled` FAIL |
| `NewRouter` passes a literal `false` | the gate-wired composition test reddens alone | `..._NewRouterSilentWhenGateWired` FAIL |
| remove the `d != nil` guard AND `Policy()`'s nil check | the typed-nil test reddens | FAIL with a nil dereference |
| restored | green | ok |

The third row is the one that matters most. The first four tests exercise the
helper directly, so they would ALL pass on a correct helper that nothing calls —
the same shape of defect as the silent no-op this ticket fixes. Only the
`NewRouter` test rules that out, and the mutation confirms it does.

The second row is the other one worth keeping: a check that warned whenever
`reject` is configured, regardless of wiring, would satisfy the primary
acceptance criterion and be actively harmful — an operator who learns to ignore
a false alarm gets nothing from it in the deployment where it matters.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows the `provision` warning in `internal/acl/declarative.go:208` — same
mechanism, same class of problem (a configured mode that does not do what its
name suggests). Reusing that channel rather than inventing one means an operator
already watching for one of these sees the other.

DRY: nothing to extract. The two warnings live in different packages and say
different things; sharing them would need a parameterised message that reads
worse than either.

Security: this changes no authorization decision — it reports one. The value is
entirely in closing the gap between what an operator believes is enforced and
what is. The message carries no principal, token or entity data.

The warning also gives the "SetJWTGate MUST run before NewRouter" ordering
invariant a runtime voice, which it previously did not have: it was protected
only by a code comment, and a refactor that reordered them failed silently. It
now warns. That is a side benefit, not the ticket's purpose, and it is noted in
the function's godoc so the next reader knows the check is doing two jobs.
