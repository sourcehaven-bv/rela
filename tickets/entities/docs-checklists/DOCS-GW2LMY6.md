---
id: DOCS-GW2LMY6
type: docs-checklist
title: 'Docs: LLM actions on collection screens'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: batchRows (two-pass gate and the face rule), judgeWhen (fails closed as whenHolds), runLLMBatch (log line without ids), BatchSchema ($defs sharing), useBatchAction (starting flag, confirm text), LLMBatchProposalDialog (why every change starts unticked)
- [x] Function/type docs if public API: actionproposal.Batch/Failure, llmtask.BatchRow/BatchRequest/ParseBatch/ErrBatchTooLarge, dataentryconfig.LLMBatch, Kanban.Actions

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README content covers llm actions)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing collection-read and affordance rules)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from commits)
- [x] API docs updated (if applicable): docs/data-entry.md (via GUIDE-data-entry) gains "Running on several entities (batch)": config, limits, where it runs, gating, review, answer size, timeout, the entity_ids request and the batch response
