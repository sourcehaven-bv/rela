---
id: REV-2X98H
type: review-checklist
title: 'Review: Replace lua.Services struct with minimal consumer interfaces per call site'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [ ] All tests pass (`just test`)
- [ ] Lint clean (`just lint`)
- [ ] Coverage maintained (`just coverage-check`)

## Code Review

- [ ] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [ ] All critical review-responses addressed
- [ ] All significant review-responses addressed
- [ ] Self-reviewed the diff for unrelated changes

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [ ] Each acceptance criterion tested (reference planning checklist)
- [ ] Test evidence documented in implementation checklist

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [ ] Docs-checklist created and linked via `has-docs`
- [ ] User-facing documentation updated
- [ ] Docs-checklist marked as done

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [ ] Commit message explains the why, not just what
- [ ] No TODOs or FIXMEs left unaddressed
- [ ] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- e.g., https://github.com/org/repo/pull/123 -->
