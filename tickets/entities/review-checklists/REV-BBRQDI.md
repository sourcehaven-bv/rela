---
id: REV-BBRQDI
type: review-checklist
title: 'Review: Read-only graph access in Lua migration steps: rela.get_entity and rela.get_relations'
started: "2026-10-13"
completed: "2026-10-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — via `just coverage-check`'s full run: every
package passes except `cmd/rela-desktop` `TestChromeStyle_TargetsShippedClasses`
("no stylesheets in the embedded SPA"), which needs a built frontend this fresh
worktree does not have; unrelated to this change. `TestLuaReads_*` also run
against PostgreSQL locally.
- [x] Lint clean (`just lint`) — all four tag sets, 0 issues
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) — total 83.5%, thresholds pass

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed — none
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-27D1J7 RR-4WUC0Y RR-5D22NQ RR-5H08R1 RR-724GLI
RR-9AMRA7 RR-BXSM9E RR-C00RAC RR-KBQDGA RR-L71X0C RR-OZEBIF RR-WDVJ0X RR-YGCP6K.
Four significant, all addressed. One minor (dry-run warning in the CLI) deferred
to TKT-RRKUNE.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. Classify by relation — PASS, `TestLuaReads_ClassifyByRelationOnEveryBackend`, CLI run.
2. Pre-step reads in any order — PASS, `TestLuaReads_SeePreStepStateInEitherOrder`
(incl. reversed order), `TestLuaReads_OwnRowIsPreStep`; fails with a
write-as-you-go mutant.
3. Later step sees earlier step — PASS, `TestLuaReads_LaterStepSeesEarlierStep`.
4. Idempotent re-run — PASS, second run affects 0 (test and CLI).
5. No ACL/validation — PASS, `TestLuaReads_IgnoreSchemaAndValidation`.
6. `entity: "*"` — PASS, `TestLuaReads_WildcardStep`.
7. All backends — PASS on memstore, fsstore, sqlitestore, pgstore.
8. No writes — PASS, `TestLuaReads_OnlyReadsAreExposed`.
9. Faces — PASS, `TestLuaReads_Faces`, `TestLuaReads_StepFaceAndFaces`
(adds `get_faces` and `e.face` per review).
10. Bad arguments — PASS, `TestLuaReads_RejectBadArguments`, `TestLuaReads_StoreErrorRaises`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-S75WFR

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
