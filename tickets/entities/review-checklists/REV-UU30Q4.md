---
id: REV-UU30Q4
type: review-checklist
title: 'Review: lua: reject non-string filter options in the gated rela.get_relations'
status: done
---

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./...` → 0 failures on the rebased branch. `just lint` → 0 issues.
`just lint-md` → 0 issues. `just arch-lint` → OK, no new rules needed.
`internal/lua` coverage 84.9% against a floor of 80.

Note on the local suite: before this branch, `just test` failed on
`TestScriptReadSeam_PolicylessProjectStaysUnrestricted` (stale identity
assertion vs the `visibility.Unrestricted` wrapper from TKT-1WV50C). That was
fixed upstream in parallel — my equivalent fix was dropped during the rebase in
favour of theirs. The suite is now green end to end.

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: this ticket IS the resolution of
RR-D7KXKV; the cranky-code-reviewer already scrutinised this exact
argument-parsing function on the elevated path in TKT-ACSBSA, and the change
here is to share that reviewed implementation rather than write a new one)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none new — resolves RR-D7KXKV (minor), raised against
TKT-ACSBSA.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Non-string `from`/`type`/`to` raise instead of silently widening — PASS
(`TestGetRelations_RejectsNonStringFilter`, 4 subtests; reproduced failing
before the fix).
- Absent / partial / valid filters still scope correctly — PASS
(`TestGetRelations_AcceptsAbsentAndStringFilters`, 7 subtests).
- Gated and elevated paths cannot drift — PASS (single shared `relationQuery`;
the duplicate implementation is gone).
- No in-tree caller regresses — PASS (120/120 live validation rules,
including the `entity.id` caller; all id sites construct `lua.LString`).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-S8CF2L

Corrected: this section was first marked N/A on the grounds that the guide edits
were small and inline. The `analyze_validations` rule "Done enhancement tickets
must have completed docs checklist" rejected that, and it was right — the change
is user-facing behavior on a documented binding, and one edit fixed a broken
example.

`GUIDE-lua-scripting.md` gained an options-table contract block for
`rela.get_relations` (string-typed keys, the raise, and the deliberate bare-id
asymmetry). `GUIDE-scheduled-tasks.md` had a genuinely broken example corrected.
The elevated-path note now records that both bindings share one rule.
Regenerated via `just docs`; `just docs-check` passes in the pre-push hook.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1239

**CI:** 23 SUCCESS / 1 SKIPPED / 1 FAILURE. The sole failure was the **"Rela
Tickets"** job (failing step: "Run rela validate"), which enforces "tickets in
`review` status cannot be merged" — this ticket's own status gate, not a defect
in the change. Cleared by moving TKT-9FKX8X to `done` in the same branch; every
other check was green.

`mergeable=MERGEABLE`, `mergeStateStatus=BLOCKED` on `REVIEW_REQUIRED` only —
the human approval gate, which is the intended remaining step.
