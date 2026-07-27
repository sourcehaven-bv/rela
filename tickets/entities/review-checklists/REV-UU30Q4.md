---
id: REV-UU30Q4
type: review-checklist
title: 'Review: lua: reject non-string filter options in the gated rela.get_relations'
status: pending
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

- [ ] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none — this ticket IS the resolution of RR-D7KXKV, raised
against TKT-ACSBSA. `/code-review` not yet run for this follow-up; the change is
one shared argument-parsing function that the prior review already scrutinised
on the elevated path.

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

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: docs changes
are two short guide edits made inline, not a separate docs workstream)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: none created)

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

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** pending
