---
id: REV-4VL8LY
type: review-checklist
title: 'Review: Scannable detail-page field layout: single-column default + authored span (views and forms)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test -race` ok across dataentry / dataentryconfig / apiwire; `golangci-lint
run` **0 issues**; frontend **1479/1479** (89 files); `vue-tsc --noEmit` clean;
prettier and markdownlint clean.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-PR1TQK (minor, addressed).

**The review agent stalled mid-run** (API error) while investigating CSS
specificity — it had got as far as wanting to verify scoped-vs-global precedence
empirically. Rather than record a clean review that never happened, I completed
that audit by hand against the running app and filed the result as RR-PR1TQK.
Summary: the two remaining scoped rules compile to attribute-qualified
selectors, so they outrank the shared sheet *by specificity* (deterministic, and
the intended relationship), and only one rule in the whole cascade now sets
`grid-column` on a form field — so the equal-specificity hazard is gone by
construction rather than by luck.

Design-review findings from the parent ticket that this PR implements: RR-OYENHV
(both DTO sites + forms in scope), RR-IUMZV8 (loud load-time validation),
RR-P2QU85 (row behaviour specified). All satisfied.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all 10 PASS — see IMPL-U1JZ2B for per-criterion evidence.
Highlights:

- **AC 4 caught a real bug.** The both-DTO-sites test failed on first run
because I had refactored one construction site and not the other — spans worked
on cards and vanished on the detail page. Precisely the failure RR-OYENHV
predicted.
- **AC 2** verified by measuring rendered row membership from
`getBoundingClientRect()`, not by eyeballing a screenshot.
- **AC 3** documented error string verified byte-exact against real output.
- **AC 6** narrow-viewport collapse confirmed at 560px.

**A second bug was found and fixed during verification**, not by tests:
converting `.form-fields` to a grid broke the form, because `FormFieldList` also
emits relation widgets that carry their own root class. The first fix then
silently swallowed every authored span through an equal-specificity collision.
Both were diagnosed from computed style — `--field-span: 4` present while
`grid-column` computed to `span 12`. Worth stating plainly: the unit tests were
green through both of those.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` — DOCS-AH7A40
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

`docs/data-entry.md` gains a public `span` key, so unlike PR 1 this genuinely
needed user-facing docs.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** pending — opening after PR 1 (#1281) merges, since this branches off it.
