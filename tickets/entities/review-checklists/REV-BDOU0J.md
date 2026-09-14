---
id: REV-BDOU0J
type: review-checklist
title: 'Review: Guarded copy into the bare face is refused unless the caller can already write the target'
status: done
---

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./...` clean. `just lint` 0 issues. `just arch-lint` no warnings. `just
comment-lint` clean (no unresolvable doc links across 13899 comments). `just
coverage-check`: package floor (50%) PASS, total (65%) PASS, total coverage
79.3%.

**Comment findings.** `just comment-report` reports one advisory finding in a
touched file: `internal/entitymanager/copy.go:251` (`duplication`, shared with
`internal/metamodel/types.go:798`). It is pre-existing and sits outside every
region this diff changes (copy.go 407+, 469+, 496+). Not introduced here, so not
grown and not suppressed.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

Both cranky-code-reviewer and rela-security-reviewer were run. The security
review's verdict was that the loosening is safe to merge, with one minor doc
correction, which is applied.

The self-review confirms the behavioural change is four lines in
`authorizeCopy`; everything else in the diff is comments, tests and docs. No
unrelated changes.

**Review Responses:** RR-GSYM8Y (critical), RR-UKHCVD (significant), RR-FHNZFY
(significant), RR-S57CAY (minor), RR-17Z70H (minor) — all `addressed`.

The critical one was a genuine regression this change introduced: keying the
exemption on `IsSameEntity()` alone admitted `from: t` / `to: t`, a copy
crossing no face boundary, which became an unauthorized in-place write.
Reproduced, fixed by adding the `sourceTail != targetTail` clause, and pinned.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| Criterion | Status | Evidence |
| --- | --- | --- |
| Guarded same-entity copy into the bare face succeeds on the guard alone | PASS | `TestCopy_GuardIsTheAuthorizationForASameEntityCopy/holding_the_guard_suffices…`, under `acl.ReadOnlyACL` with a precondition asserting the direct edit is refused |
| Missing guard permission → `ForbiddenError` with `RuleKind: "copy-guard"` | PASS | same test, `lacking_the_guard_is_refused_even_under_an_allow-all_ACL` |
| Unguarded same-entity copy into the bare face still refused | PASS | `TestCopy_IntoTheBareFaceNeedsUpdate` (pre-existing, still green) |
| Guarded cross-entity copy still needs create/update on the target | PASS | `TestCopy_GuardDoesNotOverruleACrossEntityWrite`, asserting `RuleKind == "test"` so only check (3) satisfies it |
| Guard does not substitute for reading the source | PASS | same test, `the_guard_never_substitutes_for_reading_the_source`, with the gate recording that it was consulted |
| Guarded same-face copy still refused (found in review) | PASS | `TestCopy_GuardDoesNotOverruleASameFaceCopy` |
| Affordance hint agrees with the write | PASS | `TestCopiesForSource_GuardedCopyIntoTheBareFaceIsOffered` |

Every clause of the condition was mutation-tested: deleting any one fails a
distinct test.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, not
an enhancement)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: none created)

`docs/content-states.md` stated the old rule in prose, so it was wrong rather
than merely incomplete and had to be corrected as part of the fix. It now covers
all four cases, and adds a warning that `fields: all` is a full replace rather
than a merge — which matters precisely for the `bare_face: published` shape this
change enables, since that target also carries `unique:` keys.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] ~~Run `/pr` command to create PR and monitor CI~~ (not run: opening a PR
is outward-facing and was not requested; awaiting the go-ahead)
