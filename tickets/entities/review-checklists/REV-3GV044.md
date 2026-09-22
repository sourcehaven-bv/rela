---
id: REV-3GV044
type: review-checklist
title: 'Review: Form save drops all relation edits when the entity has a relation the form does not render'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (196 files / 3182 tests; `just arch-lint` clean)
- [x] Lint clean (`npm run lint`: 0 errors; `vue-tsc --noEmit` clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained~~ (N/A: frontend-only change; the frontend has no
coverage enforcement — see CLAUDE.md "Coverage")

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-820N22 (critical, addressed), RR-4B7VVC (critical,
addressed), RR-FL9F28 (significant, addressed), RR-UUDJGE (minor, addressed).

The review also found a pre-existing defect this change neither introduces nor
worsens — an incoming form relation whose type declares no inverse throws at
submit, because config validation, the serializer's `_inverse` fallback and the
SPA's `getInverseName` disagree. Filed as BUG-2XN24C rather than fixed here.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Editing a relation on a form whose entity carries an unrendered relation
saves — PASS. Browser-verified against atlas; no toast, 11 -> 12 chips, PATCH
carried 12 typed identifiers and no `onderdeel_van`.
- The unrendered relation is not asserted by the form — PASS. Absent from the
body; server preserved it.
- No prefill channel loses an edge silently — PASS. `rel.*`, `link_as: to` and
Duplicate are all exempted; each pinned by a test verified to fail without its
exemption.
- The error names the relation and gives the right remedy — PASS. Two wordings,
one per cause; the prefill case no longer advises a reload.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: no behaviour a user configures)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)
