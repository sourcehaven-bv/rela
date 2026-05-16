---
id: REV-Z4GK
type: review-checklist
title: 'Review: Migrate scheduler to wire its own services (off Workspace)'
status: done
---

## Code Review

- [x] cranky-code-reviewer run on the diff
- [x] 1 critical finding addressed (State() moved from cliRead to cliWrite)
- [x] 1 significant finding addressed (schedulerProvider adapter dropped — cliWrite satisfies WorkspaceProvider structurally)
- [x] 3 minor findings verified/acknowledged
- [x] Tests pass under `-race`
- [x] `just ci` green

## Disposition

Reviewer caught two real design errors in the first draft:

1. **`State()` on `cliRead` was a lie.** state.KV exposes Put/Delete — putting that on a "read" bundle defeats the read/write split. Moved to `cliWrite` with a clarifying comment.

2. **Adapter was producer-side-interface anti-pattern in disguise.** `schedulerProvider` held a wide `cliWrite` reference behind a narrow facade — same coupling, just one indirection. Dropped entirely; `cliWrite` happens to expose Paths/Config/State/LuaWriteDeps, so it satisfies `scheduler.WorkspaceProvider` structurally. Zero adapter code.

Final shape is the cleanest version of the migration: scheduler command flows
through the same PersistentPreRunE wiring as every other subcommand; the
scheduler-side `WorkspaceProvider` interface is the only contract that has to
exist.

See IMPL-B6ZF for the full table.
