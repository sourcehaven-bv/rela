---
id: PLAN-KIOA4C
type: planning-checklist
title: 'Planning: Warn when unmatched_principal reject has no JWT gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: a load-time `slog.Warn` at `NewRouter` when `unmatched_principal: reject` is
configured and no JWT gate is wired.

OUT, with reasons:

- *Making it a hard error.* Refusing to start turns a wiring omission into an
outage for a deployment that is merely no stricter than the default
(`anonymous`) — the posture it had before the key existed. The operator's
mistake is BELIEVING a restriction is in force; the fix is to say so, not to
take the server down. This also matches how `provision` already reports its own
inert state.
- *Moving the check into `Policy.Validate`.* It cannot see server wiring. That
is the whole reason the hole exists.
- *Enforcing the SetJWTGate/NewRouter ordering mechanically.* Out of scope, but
note the warning gives that invariant a runtime voice for free: a refactor that
reorders them now makes this fire, where before it failed silently.

**Acceptance Criteria:**

1. `reject` + no gate → warns, and the message says the setting is INERT.
*Test:* capture `slog`, assert the output names the setting AND contains "NO
effect". A warning that only says "reject is configured" reads as confirmation
it is working, which is the belief being corrected.
2. `reject` + gate wired → silent.
*Test:* same helper, `jwtWired=true`, assert no output. This is the
discriminating case: an operator who learns to ignore a false alarm gets nothing
from the warning in the deployment where it matters.
3. Other modes → silent regardless of wiring.
*Test:* table over `""`, `anonymous`, `provision`. `provision` has its own
warning where it would have acted; duplicating it here would double-report one
condition.
4. A nil policy is silent and does not panic.
5. `NewRouter` actually calls the check.
*Test:* build an App with a reject policy, call `NewRouter()` WITHOUT
`SetJWTGate`, assert the warning appears. AC1–4 prove the predicate; only this
proves anything invokes it.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — a single guard.

**Existing Solutions:**

- The in-repo precedent is the `provision` warning in
`internal/acl/declarative.go:208`, which reports the same class of problem (a
configured mode that does not do what its name suggests). Same mechanism
(`slog.Warn`), same reasoning, so this follows it rather than inventing a
reporting channel.
- `Policy.Validate` was checked first and rejected as the home: it validates
acl.yaml against itself and has no access to server wiring.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Add `warnUnmatchedRejectWithoutJWTGate(p *acl.Policy, jwtWired bool)` in
`internal/dataentry/router.go` and call it from `NewRouter` immediately before
`attachACLRequest`, using the same `a.jwtGate != nil` expression that
`attachACLRequest` receives. Reading the same expression is deliberate: a check
computed differently could disagree with the thing it is describing.

It takes the policy and the wiring bool rather than the `*App` so it is a pure
function, testable without constructing a server.

**Alternatives considered:**

- *Hard error at load.* Rejected — see Scope.
- *Check inside `attachACLRequest`.* Rejected: that runs per REQUEST, so the
warning would repeat on every call and be lost in noise. This is a startup fact
and belongs at startup.
- *Return an error from `NewRouter`.* Rejected: it does not currently return
one, and changing that signature for a warning is a large blast radius for a
low-severity diagnostic.

**Files to modify:**

- `internal/dataentry/router.go`
- `internal/dataentry/unmatched_reject_wiring_test.go` (new)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `unmatched_principal` comes from `acl.yaml` and is already validated to an
allowlist of three modes by `Policy.Validate`; this reads the effective value
via the existing `EffectiveUnmatchedPrincipal()` so the empty default resolves
the same way it does everywhere else.
- The wiring bool is internal state, not input.

**Security-Sensitive Operations:**

This changes no authorization decision. It reports one. The security value is
entirely in closing the gap between what an operator believes is enforced and
what is.

The message contains no principal, token, or entity data — only the name of the
setting and what to do about it.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one per acceptance criterion.

AC5 is the one that makes the rest mean anything. AC1–4 exercise the helper
directly and would all pass on a correct helper that NOTHING CALLS — which is
the same shape of defect as the silent no-op being fixed here.

**Edge Cases:**

- nil policy (AC4).
- empty-string mode → resolves to `anonymous`, silent (AC3).
- `provision` → silent here; it has its own warning (AC3).

**Negative Tests:**

AC2 and AC3 are the negatives, and AC2 is load-bearing: a check that warns
whenever `reject` is set — regardless of wiring — would satisfy AC1 and be
actively harmful, training operators to ignore it.

Mutation plan: (a) make the guard always return early → AC1 and AC5 redden; (b)
drop the `jwtWired` term → AC2 reddens; (c) delete the call from `NewRouter` →
AC5 reddens alone.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Checking a proxy for the condition instead of the condition.* MISSED in
planning, found in review (RR-0CJ47L). `reject` needs THREE things — the mode, a
wired gate, and the `principal_property` lookup — and the plan reasoned only
about the gate, because that is the one the issue named. A reject policy without
the lookup was equally inert and warned about nothing: a second silent state, in
exactly the shape of the bug being fixed.

The plan even recorded the counter-argument and got it wrong, asserting that
`Policy.Validate` refuses that combination. It does — but `NewDeclarative` never
calls `Validate`, so any construction path skipping `LoadPolicy` reaches it,
including this ticket's own test. Mitigated by extracting
`acl.Policy.RejectEffective`, so the warning and the enforcement site read one
definition and cannot drift.

The lesson for a warning specifically: it must check the condition the
enforcement site EVALUATES, not the condition the ticket describes. A warning
that is wrong about the thing it warns about is worse than no warning.

- *Proving the call site calls something is not proving it calls it right.* Also
missed (RR-U5LKGQ). AC5 was written to rule out a correct-but-uncalled helper,
and did — but it asserted only that a warning appeared, so substituting a
literal `false` for the wiring argument left the suite green. The same omission
the plan was congratulating itself for avoiding, one level up.

- *Warning fatigue.* A false alarm is the main way this change could make things
worse. Mitigated by AC2/AC3 and their mutation checks — the warning fires for
exactly one condition.
- *Correct-but-dead code.* Mitigated by AC5 going through `NewRouter`.

**Effort:** xs

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/acl-security.md` documents `unmatched_principal`. Whether it should
state the JWT-gate dependency is a real question, to be settled in the
docs-checklist — the warning helps an operator who already deployed, and the doc
helps one who has not. NOTE: `docs/` is GENERATED from `docs-project/` entities;
edit the source, not the output.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none critical or significant. The decision worth
recording is warn-not-fail, and its reasoning is in Scope.
