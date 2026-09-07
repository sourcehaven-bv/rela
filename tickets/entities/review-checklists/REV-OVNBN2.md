---
id: REV-OVNBN2
type: review-checklist
title: Review
status: done
---

## Automated Checks

- [x] All tests pass (`go test -race ./...`, enforced by the pre-commit hook)
- [x] Lint clean (`just lint` — 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check` — 78.9% total, all floors pass)

Also clean: `just arch-lint` (the new `appbuild → entitymanager` import is
already permitted — appbuild is the composition root) and `just plimsoll`.

## Code Review

- [x] Run `/code-review` (cranky-code-reviewer + rela-security-reviewer)
- [x] All critical review-responses addressed — none were raised
- [x] All significant review-responses addressed (RR-SAXKLP, RR-Q7836T)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-SAXKLP, RR-Q7836T, RR-ZVM78B, RR-DPYJ3V, RR-UEP3A0,
RR-37NMKG

The security review returned **no findings**; it independently enumerated the
four `acl.ACL` implementations, confirmed the guard cannot be bypassed by a
policy expressed as a different type, confirmed no production path reaches the
new allow-all opt-outs, and confirmed the gate construction move is semantically
identical at both call sites.

The general review raised no critical issues and two blocking ones, both fixed:

- **RR-SAXKLP** was the one worth the whole review: the opt-out I added to
satisfy the guard could itself be constructed broken (`AllowAllCopyVisibility{}`
with a nil store passed `New` and nil-panicked at copy time) — the same
deferred-downstream-symptom failure the guard exists to prevent. Fixed by
unexporting the field behind a validating constructor, so the broken value is
now unconstructible rather than merely detected.
- **RR-Q7836T**: the two field godocs still stated the old unconditional
nil-tolerance. Rewritten to the repo's `Nil:` convention.

RR-37NMKG is **deferred with a documented reason** (a composition-root refactor,
out of scope for a bug fix; the reviewer scoped it the same way). No open
critical or significant responses remain.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- *A policy-backed `Deps` with either copy gate nil is refused* — **PASS**
(`TestNew_CopyGatesRequiredUnderPolicy`, both-nil and half-nil subtests).
- *The no-policy tier is undisturbed* — **PASS** (same test, `NopACL` and
`ReadOnlyACL` subtests both build without the gates).
- *Explicit opt-out is accepted* — **PASS** (same test).
- *The unsafe state was real* — **PASS**: reverting the fixture's two gate
lines fails `./internal/appbuild/...`; restoring goes green.
- *The guard is not vacuous* — **PASS**: the reviewer independently mutated the
guard away and both new tests failed.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, no
user-facing surface changes)
- [x] ~~User-facing documentation updated~~ (N/A: constructor contract and
wiring only; no CLI, API or config change)
- [x] ~~Docs-checklist marked as done~~ (N/A: see above)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
