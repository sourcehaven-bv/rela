---
id: REV-EXK2MG
type: review-checklist
title: 'Review: Warn when unmatched_principal reject has no JWT gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just lint` 0 issues (after fixing two `misspell` findings — British spellings
in new comments); `just arch-lint` clean; `just comment-lint` no unresolvable
doc links; `just plimsoll` clean; `just lint-md` 0 issues. `just test` green
under `-race -shuffle=on`.

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-0CJ47L (critical), RR-U5LKGQ, RR-I1RTSU, RR-FZ1CBE
(significant). All addressed.

The critical one is the ticket's own lesson turned back on the fix. `reject`
needs THREE conditions, not one: the mode, a wired JWT gate, AND the
`principal_property` lookup. My warning checked the gate alone — so a reject
policy without `user_entity_type` / `principal_property` was equally inert and
warned about nothing. A second silent-inert state, uncovered, in exactly the
shape of the bug being closed.

I had dismissed that case on the grounds that `LoadPolicy` refuses it. It does,
but `acl.NewDeclarative` never calls `Validate`, so any construction path
skipping `LoadPolicy` reaches it — including this ticket's own test. The
generalizable error: I wrote a check for the condition I had in mind rather than
for the condition the enforcement site evaluates.

Fixed structurally, with `acl.Policy.RejectEffective(jwtWired bool)` as the one
definition both sites read, so the warning cannot be wrong about the thing it
warns about.

The other three:

- **No test pinned the argument `NewRouter` passes** (RR-U5LKGQ). Substituting a
literal `false` at the call site left the suite green — the composition test
asserted only that *a* warning appeared. This is the same mistake one level up
from the ticket: I had proof the call site calls something, and mistook it for
proof it calls it correctly.
- **The new `d.Policy()` call widened a typed-nil's blast radius** (RR-I1RTSU) —
`NewRouter` would panic at construction where a typed-nil previously failed
later, per request. Made `Policy()` and the predicate chain nil-receiver safe,
and pinned it.
- **The slog capture buffer was unsynchronized** (RR-FZ1CBE), against a
precedent in `appbuild` that had already documented the hazard and its fix.

Two `misspell` findings (British spellings) were fixed. A `TKT-*` placeholder
that had shipped in two comments was replaced with the real id.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | `reject` + no gate → warns, message says INERT | PASS | `TestUnmatchedReject_WarnsWhenNoJWTGateWired` |
| 2 | `reject` + gate wired → silent | PASS | `TestUnmatchedReject_SilentWhenJWTGateWired` |
| 3 | other modes → silent regardless of wiring | PASS | `TestUnmatchedReject_SilentForOtherModes` |
| 4 | nil policy → silent, no panic | PASS | `TestUnmatchedReject_NilPolicyIsSilent` |
| 5 | `NewRouter` actually calls the check | PASS | `TestUnmatchedReject_NewRouterWarnsWhenGateMissing` |
| 6 | `reject` + gate + NO lookup → warns, naming the lookup | PASS | `TestUnmatchedReject_WarnsWhenLookupDisabled` (added from RR-0CJ47L) |
| 7 | `NewRouter` passes the RIGHT wiring argument | PASS | `TestUnmatchedReject_NewRouterSilentWhenGateWired` (added from RR-U5LKGQ) |
| 8 | a typed-nil `*Declarative` does not panic `NewRouter` | PASS | `TestUnmatchedReject_TypedNilDeclarativeDoesNotPanic` (added from RR-I1RTSU) |

AC6–8 all came out of review, not planning. Each is mutation-verified to redden
alone, which is what shows they cover distinct cases rather than restating
AC1–5.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-WIU04Z (done).

`docs/acl-security.md` gains a bullet in the `unmatched_principal` section's
"Load-bearing details" list. Review caught that the first placement referred to
the missing-lookup case as "above" when it was below, and that calling this "not
a load error" invited the reader to conclude the lookup requirement is enforced
wherever a `Declarative` exists — it is not, since `NewDeclarative` skips
`Validate`. Both fixed: the bullet now follows the lookup one and refers back to
it.

Edited in `docs-project/` and regenerated with `just docs`. `docs/` is GENERATED
— editing the output directly fails the Docs CI check, as the sibling ticket
TKT-LVSPSB discovered earlier in this round.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
