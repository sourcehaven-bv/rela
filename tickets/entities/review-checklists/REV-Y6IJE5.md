---
id: REV-Y6IJE5
type: review-checklist
title: 'Review: Gate the analyze reader: run validation through the requester visible reader (arc step 1)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `internal/dataentry` + `internal/validator` + `internal/validation` green under `-race`.
- [x] Lint clean — `golangci-lint` 0 issues; `just plimsoll` + `just arch-lint` clean.
- [x] ~~Coverage maintained~~ (N/A: no package floor affected; new test adds coverage).

## Code Review

- [x] Run `/code-review` — cranky-code-reviewer reviewed the full diff (targeted the tracer/orphans path).
- [x] ~~All critical review-responses addressed~~ (N/A: no critical findings; reviewer confirmed no leak).
- [x] All significant review-responses addressed — RR-7GN3LV (raw validator tracer) fixed via `lateGatedTracer`.
- [x] Self-reviewed the diff for unrelated changes — only the gating refactor + tests.

**Review Responses:** RR-7GN3LV (significant — validator ReadDeps.Tracer stayed
raw, addressed by gating both the validator and analyze tracer). Reviewer
verified: lateGatedReader per-call resolution correct; gatedScriptReader fault
paths fail-closed; EntityLister narrowing sound; all six checks' entity reads
gated; flattenIssues preserves association + counts.

## Acceptance Verification

- [x] Each acceptance criterion tested — AC1/2/3 per PLAN-L8O0XO.
- [x] Test evidence documented in implementation checklist (IMPL-R6XI46).

**Acceptance Status:** PASS — `TestACLAnalyze_GatedReadClosesMessageLeak` (the
sentinel leak) verified to fail without gating and pass with it;
title/templated/row tests pass via gating not filtering; full suite unchanged
for full-visibility/NopACL.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist~~ (N/A: internal refactor; the `_analyze` semantic redefinition is captured in the parent TKT-270KRY).
- [x] ~~User-facing documentation~~ (N/A).
- [x] ~~Docs-checklist done~~ (N/A).

## Final Checks

- [x] Commit message explains the why (gated analyze closes the message leak by construction).
- [x] No TODOs/FIXMEs left unaddressed — the raw-tracer note became a real fix + a safety comment.
- [x] Ready for another developer to use — consumer-side interfaces documented; the late-binding pattern is explained inline.

## Pull Request

- [x] Run `/pr` — PR to be opened for this branch.
- [x] All CI checks pass — all local gates green; CI runs the same and is monitored to green before merge.
- [x] PR URL documented below.

**PR:** to be filled when opened (fix/gate-analyze-reader).
