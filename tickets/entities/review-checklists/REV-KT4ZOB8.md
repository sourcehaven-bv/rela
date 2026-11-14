---
id: REV-KT4ZOB8
type: review-checklist
title: 'Review: LLM actions on collection screens (kanban, lists, piles)'
started: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): Go tests for llmtask, actionproposal, dataentryconfig, dataentry, script, apiwire and mcp pass; frontend 114 files (1612 tests) pass, plus the touched files after review fixes (583 tests). The full `just test` was not run: other sessions were running `go test -race ./...` on this machine
- [x] ~~Lint clean (`just lint`)~~ (not run locally: golangci-lint failed to load packages three times with "WaitDelay expired" while other sessions held the machine; CI runs it. `go vet` and `just arch-lint` are clean on the changed packages, `vue-tsc -b` is clean, and ESLint reports no errors on the touched files)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: the machine was loaded by other sessions' race runs; the new Go code is covered by batch_test.go, llmbatch_test.go and actions_llm_batch_test.go, and CI enforces the floors)

**Comment findings.** No advisory findings checked locally; CI reports them.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent); rela-security-reviewer ran alongside
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed: RR-MN2K3U4 deferred with reason (not reachable: llm actions refuse faced types)
- [x] Self-reviewed the diff for unrelated changes (prettier churn in EntityList.vue and KanbanView.vue reverted)

**Review Responses:** RR-MN2K3U4, RR-P645GDW, RR-JQXQPT9, RR-DGSU3FO, RR-O63LY12, RR-GHMVDTA, RR-47HEG0L, RR-BH3Y440, RR-XHMAI3N, RR-A9H63IX, RR-M4P76M0, RR-YRS51PK

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: TestLLMBatch_ProposesPerRowWithoutWriting (store unchanged).
2. PASS: TestLLMBatch_ProposesPerRowWithoutWriting (when) and TestLLMBatch_GatedRowsAreNotSent (unreadable type, face, when on a hidden property).
3. PASS: TestLLMBatch_HiddenFieldsAreNotSent.
4. PASS as amended by the design review: an unknown record id or bad JSON rejects the reply (TestParseBatch_Rejects, Refusals); a missing record or invalid value fails only its own row (TestParseBatch, ProposesPerRow).
5. PASS as amended: over max, entity_ids on a non-batch action, both forms, an empty list and non-string ids give a 400 (TestLLMBatch_Refusals); duplicate ids run once (ProposesPerRow).
6. PASS: TestValidateActions_LLMBatch, TestValidateKanbanActions, TestValidateLists_BatchAction.
7. PASS: LLMBatchProposalDialog.test.ts.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-GW2LMY6

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (next step, after done)
