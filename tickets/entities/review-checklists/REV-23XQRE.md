---
id: REV-23XQRE
type: review-checklist
title: 'Review: propose/commit seam'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — 108 files, 1727 tests (frontend); Go `dataentryconfig`
  green
- [x] Lint clean — 0 errors (92 pre-existing warnings, none in touched files)
- [x] Typecheck clean (`vue-tsc --noEmit`)
- [x] ~~Coverage maintained~~ (N/A: the frontend has no coverage enforcement —
  see root `CLAUDE.md`; the Go change is a two-line allowlist addition)
- [x] Verified against the rebased dependency set, including the Pinia 3 → 4
  major bump that landed on develop mid-work

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) — run
  twice: once on the seam, once adversarially on the whole implementation
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses.** Design review (before implementation): RR-DRRC66,
RR-C5DBEB, RR-YWIN6T (critical), RR-RTZG5T, RR-CWRQPQ, RR-VXKBYC, RR-81E7VH,
RR-270F1Z (significant), RR-2SDLTF, RR-MYHDH0 (minor).

Code review (after implementation): RR-KG04FD, RR-TXMRYN, RR-HUL1JC (critical),
RR-U5ICXO (significant), RR-S8LX1S (minor). All addressed.

The second review is the one that earned its keep. It found **two data-loss
paths that the passing test suite did not**:

1. **A redacted `confirm` field was cleared with no dialog.** A field the
   principal cannot read is absent from `formData`, so it read as *empty*; the
   dialog was skipped, and `clearOnHide` cleared it anyway because it
   re-derived intent from config rather than from the decision. Reproduced:
   **0 dialogs, 1 unset**. This is BUG-FB0LN8's original symptom via the ACL
   path.
2. **A write fired after the component unmounted.** Answering the dialog after
   navigating away resumed the awaited continuation on a dead form.
   Reproduced: **1 PATCH after unmount**.

It also ran a mutation battery and showed that two mechanisms — the navigation
fence and the generation bump — were asserted by **nothing**: deleting either
left all tests green. Both now have tests that fail when the mechanism is
removed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS for AC1-AC7 of PLAN-6X0Y7W. Evidence is the
mutation table in IMPL-HF0WBT: each criterion has at least one test that fails
when its mechanism is broken.

AC2 (single atomic PATCH) is verified end-to-end through the real widget path:
an approved clear emits one request carrying both the trigger property and the
`properties_unset`.

**Not verified:** manual browser testing. Called out explicitly in
IMPL-HF0WBT — every previous `confirm` attempt passed its tests and failed
under a real click.

## Documentation (enhancements only)

- [x] User-facing documentation updated — `clear_when_hidden: confirm`
  documented at source (`docs-project/entities/guides/GUIDE-data-entry.md`) and
  regenerated via `just docs`
- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: the docs change
  is one section of an existing guide, updated in the same PR)
- [x] Module docs updated — `useHiddenFieldPolicy`'s docstring no longer
  describes `confirm` as unimplementable

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — 24 green
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1333 — approved by
tschmits, whose review comment (the PR description understating its scope) is
addressed in this revision.
