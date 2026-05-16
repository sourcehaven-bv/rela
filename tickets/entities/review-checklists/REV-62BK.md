---
id: REV-62BK
type: review-checklist
title: 'Review: Lift workspace.AttachFile / ListAttachments to internal/attachment'
status: done
---

## Code Review

- [x] cranky-code-reviewer run on the diff
- [x] No critical findings
- [x] 3 significant findings addressed in-PR (ctx threading, orphan-on-update-failure, error wrapping)
- [x] 4 minor findings addressed (doc tense, storeStub doc, map→slice, deterministic findFileProperty)
- [x] 2 leverage findings deferred (narrow EntityManager interface — revisit after all 3 lifts land)
- [x] Tests pass under `-race`
- [x] `just ci` green

**Summary:** see IMPL-PSKK for the full disposition table.

**Code Review Summary:**

Cranky review said the lift was "mechanically correct and gated by every check"
— no critical issues. Real wins from the review:

- **Context plumbing.** All 3 lifts share the same `context.Background()` smell from the workspace facade methods; lifting now is the cheapest moment to thread `ctx` through. Done: `Attach(ctx, ...)` / `List(ctx, ...)`. CLI subcommands pass `cmd.Context()`.
- **Orphan-file visibility.** A failure in `UpdateEntity` after `Store.AttachFile` writes leaves the file orphaned. Error message now names the path and points at `rela gc --temp-files`.
- **`findFileProperty` determinism.** Was iterating a Go map (random order). Now alphabetical. Documented.
- **Error wrapping.** `entity not found: %s` → `get entity %s: %w`. Test now asserts via `errors.Is(err, store.ErrNotFound)`.
- **Type renames.** `AttachmentInfo` / `AttachResult` → `Info` / `Result` (revive: no stutter under package qualifier `attachment.X`).
