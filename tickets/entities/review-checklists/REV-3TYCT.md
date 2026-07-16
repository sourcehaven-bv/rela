---
id: REV-3TYCT
type: review-checklist
title: 'Review: Multi-step (wizard) forms with conditional steps'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — Go: affected packages + `go build ./...` OK; Frontend: `npm run test:run` → 1323 passed; e2e: `wizard.spec.ts` → 7 passed
- [x] Lint clean — `npx eslint` 0 errors on changed files; `just arch-lint` OK; `just plimsoll` OK
- [x] ~~Coverage maintained~~ (frontend has no coverage enforcement; Go floors unaffected — new packages are well-covered)

## Code Review

- [x] Run `/code-review` — cranky-code-reviewer invoked on the full wizard diff
- [x] All critical review-responses addressed — RR-O4SRG (create-path hidden-branch leak)
- [x] All significant review-responses addressed — RR-ERCH9 (e2e gap), RR-HMWK7 (default divergence), RR-JBQAC (validation-scope affordance)
- [x] Self-reviewed the diff — scoped to wizard files; single-page path unchanged (existing `forms.spec.ts` + suite green)

**Review Responses:**

- RR-O4SRG (critical) — hidden-branch pruning bypassed on create path → `pruneWizardHidden` applied by both paths; visible_when-hidden wins over userTouched; regression e2e added.
- RR-ERCH9 (significant) — e2e didn't exercise the leak → added reveal→fill→hide→submit test.
- RR-HMWK7 (significant) — metamodel default in no step dropped → `managedProperties`; prune only managed-but-inactive keys.
- RR-JBQAC (significant) — wizard validation scope ignored affordance filter → use `visibleStepFields` in submit + Next scope.
- RR-8SXTZ (minor) — duplicate step-title key collision → key stepper on index.
- RR-JNNN6 (minor) — all-hidden-steps showed Submit over empty form → footer gated on currentStepDef + "No steps" message.
- RR-YKAS8 (minor) — client-only enforcement discoverability → reconciled in one place (`pruneWizardHidden`) + docs note.
- (Reviewer finding #7, allocation on empty input, was a nit the reviewer said not worth changing — no change.)

## Acceptance Verification

- [x] Each acceptance criterion tested (see IMPL-2BYV8 AC mapping)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** All 5 ACs PASS (verified by `e2e/tests/wizard.spec.ts`, 7
tests):
1. Ordered titled steps — PASS ("renders steps…").
2. Show/hide + required by earlier field — PASS ("visible_when reveals…", "required_when blocks Next…").
3. Next/back + per-step validation + full validation on submit — PASS ("next/back…", "required_when blocks…", "submits a wizard…").
4. Step in URL, refresh-safe — PASS ("refresh returns…", `?step=` assertions).
5. Single-page unchanged, opt-in — PASS (existing suite green; wizard path strictly additive behind `steps`).

Plus hidden-branch pruning verified on create ("drops hidden-branch values" + "a
revealed-then-hidden field is NOT persisted on create").

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` — DOCS-X43XD
- [x] User-facing documentation updated — docs/data-entry.md "Multi-step (wizard) forms" section
- [x] Docs-checklist marked as done — DOCS-X43XD is `done`

**Docs Checklist:** DOCS-X43XD

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — `steps:` config documented; author-time `rela validate` catches condition mistakes

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending -->
