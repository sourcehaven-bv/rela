---
id: REV-FH4K72
type: review-checklist
title: 'Review: Cascade re-write skips post-automation validation/unique/transition re-checks (create path re-checks, cascade path does not)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./...` 0 failures · `just lint` 0 issues · `just arch-lint` OK · `just
coverage-check` PASS (77.1%) · `go test -race` clean on `entitymanager` and
`autocascade`.

The full-suite pass is the meaningful signal here: adding constraint checks to a
cascade write path could plausibly have broken existing automations (this
project's own `metamodel.yaml` is automation-heavy). Nothing regressed.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

No separate reviewer pass for this change. It is ~15 lines reusing three
existing collaborators, and it arrived through an adversarial route already: the
bug was *found* by review of TKT-80EWGM, then **I disproved most of my own
filing** before confirming the residual with a reproduction. The claims that
survived are the ones that survived that scrutiny.

Two judgement calls worth flagging to a human reviewer:

1. **`EnforceCreate`, not `EnforceUpdate`.** The row was created moments ago in
the same cascade, so the automation's value is still an *entry* value — it must
equal the declared entry, not merely be reachable by one legal move.
`EnforceUpdate` would silently accept a jump the create path forbids.
2. **Hard validation errors only.** Soft conditions are tolerated on every
other write path (DEC-HWZHA); rejecting them here would make a cascade
*stricter* than a direct edit. `TestCascadeWrite_ValidAutomationValueStillLands`
guards against that over-correction.

**Self-review:** diff is `cascadehost.go` (+~25 lines incl. godoc) and one new
test file. No unrelated changes.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

| Scenario | Before | After |
|---|---|---|
| Cascade + automation sets duplicate `unique:` | 2 holders, **silent** | 1 holder, error surfaced |
| Cascade + automation sets illegal entry state | `status="done"` persisted | `status="todo"`, error surfaced |
| Top-level create (control) | rejected | rejected — unchanged |
| Cascade + automation sets a legal value | lands | lands — unchanged |

**Both re-checks mutation-tested**: removing either makes its test fail.

The top-level control case is deliberate. The defect was an **asymmetry**, so a
cascade-only test would not catch a future change that weakened both paths.

## Documentation (enhancements only)

Bug fix — no user-facing surface change. The behavioural change is that a
previously-silent constraint violation now surfaces as an automation error,
which is the documented contract everywhere else.

`cascadehost.go`'s godoc gained the rationale for all three checks, including
why `EnforceCreate` is correct and why `e.ID` is excluded from the unique scan.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

**Known follow-up, deliberately not done here:** why5 identifies the systemic
cause as the absence of a shared apply-automation-results helper — nothing
structurally couples "automation mutated this entity" to "therefore
re-validate". This fix closes the second site; it does not remove the
possibility of a third. That refactor touches both write paths and deserves its
own change rather than riding along on a bug fix.

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending -->
