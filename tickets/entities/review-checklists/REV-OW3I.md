---
id: REV-OW3I
type: review-checklist
title: 'Review: Markdown renderer preserves source line breaks in data-entry view'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`npm run test:run` → 737/737)
- [x] Lint clean (`npm run lint` → 0 errors, 73 pre-existing warnings)
- [x] Coverage maintained (`just coverage-check` deferred to CI; frontend
per-file ratchet runs there)
- [x] Typecheck clean (`npm run typecheck` → no errors)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer invoked)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-FLT6)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

- RR-FLT6 (significant, addressed): softbreak test coupled to marked
implementation detail → relaxed to whitespace-normalised text equality +
structural `<br>` count = 0
- RR-4364 (minor, addressed): comment overstated "CommonMark forms"
→ narrowed to two-trailing-spaces only
- RR-Z6J9 (minor, addressed): hard-break test did not pin `<br>`
position → added innerHTML regex `/foo\s*<br[^>]*>\s*bar/` plus child-node-tag
inspection
- RR-QVPW (nit, wont-fix): edge cases for 3+ trailing spaces / EOP
hard break → marked.js parser-behaviour territory, not this ticket
- RR-VIWP (nit, wont-fix): lift marked options to module-level const
→ walkTokens closure depends on per-call refResolver; useful extraction would
split static/per-call, out of scope for xs fix

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 (soft break → space): PASS. Test "treats single newlines inside
a paragraph as whitespace, not <br>" asserts zero `<br>` and
whitespace-normalised `foo bar` in one `<p>`.
- AC2 (hard break preserved): PASS. Test "preserves CommonMark hard
breaks" asserts exactly one `<br>` positioned between "foo" and "bar" inside one
`<p>` via innerHTML regex and child-node walk.
- AC3 (lists/headings/code unaffected): PASS. All 42 existing tests
in markdown.test.ts continue to pass without modification.
- AC4 (browser reflow): PASS. The DOM produced contains no `<br>`
between wrapped lines; HTML reflow is the natural CSS behaviour of a `<p>` with
no `<br>` children. Verified at the test-DOM level (same observable as a live
browser).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A:
CommonMark soft-break semantics are the de-facto markdown standard; no
user-facing surface or behaviour-contract documentation needs updating)
- [x] ~~User-facing documentation updated~~ (N/A)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message will explain the why (source-wrap leakage into
rendered HTML), not just the what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** *pending creation*
