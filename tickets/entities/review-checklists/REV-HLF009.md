---
id: REV-HLF009
type: review-checklist
title: 'Review: Clear all doclink findings and promote the rule to a blocking CI gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — every touched package `ok`, no failures
- [x] Lint clean (`just lint`) — EXIT=0
- [x] Comment lint gate clean (`just comment-lint`) — EXIT=0, now including
`doclink`
- [x] Coverage maintained (`just coverage-check`) — runs in `just ci`; this
change adds no Go statements (comments only), so no floor can move

Also EXIT=0: `arch-lint`, `plimsoll`, `lint-md` (252 files).

## Code Review

- [x] Run `/code-review` — reviewed the full diff, 52 files
- [x] All critical review-responses addressed — none raised
- [x] All significant review-responses addressed — none raised
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none

Reviewed against the failure modes that matter for a mass comment edit:

- **Did anything outside a comment change?** No. All 63 added Go lines start
with `//`, verified mechanically rather than by eye: `git diff -U0 -- '*.go' |
grep '^+' | grep -v '^+++' | grep -vE '^\+\s*//'` returns nothing. `go build`
and the test suite confirm it.
- **Are the qualifications correct?** Each receiver was resolved from the
actual declaration (`grep 'func (r \*PolicyReader) Filter'` etc.), not guessed
from context. Two were wrong in the original comments and are now right:
`[Set.Enforce]` → `[Set.EnforceUpdate]` and `[Declarative.ReadQuery]` →
`[Request.ReadQuery]`.
- **Does unbracketing lose meaning?** No — the symbol name stays in the prose;
only non-functional markup goes. Spot-checked the results read naturally (e.g.
`internal/acl/principallookup.go`, where the surrounding parentheses were
already there).
- **Does the gate actually block?** Tested, not assumed: reverting one fix
produced EXIT=1 plus the guidance. Without that check the promotion would be
cosmetic.
- **Is the guidance message doing its job?** It names the rule in a
copy-pasteable directive, orders fixing before suppressing, and rejects "to
unblock CI". Pinned upstream by `TestGuidance`, which asserts the
fix-before-suppress ordering rather than just the text.

**One thing I want to flag rather than bury:** an earlier audit of these
findings graded 7 of 8 samples as false positives because `grep` showed the
symbols exist. That was wrong — they exist as *methods*, and Go still refuses to
link a bare `[Method]`. Every shape was re-verified against `go doc` on a
minimal package before any fix was applied. Worth recording because the same
mistake is easy to repeat on the next rule.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. `doclink` reports zero — **PASS** ("no unresolvable doc links across 10351
comments", was 64)
2. Gate includes it and passes — **PASS** (`-rules commented-code,doclink`,
EXIT=0)
3. Reintroducing a broken link blocks with guidance — **PASS** (EXIT=1, message
printed naming `doclink`)
4. No behaviour change — **PASS** (all 63 changed Go lines are comments)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated — N/A for `docs/`, justified in the
docs-checklist
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-ZPE7DG

## Final Checks

- [x] Commit message explains the why, not just what — leads with why advisory
failed (propagation, not new breakage) and why grandfathering was rejected
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — a broken link now fails CI with an
actionable message

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed. See TKT-UFV01M
— both post-date this checklist, and GitHub records them. -->
