---
id: REV-IVR6
type: review-checklist
title: 'Review: ScriptRunner takes Mutator per-call (delete wsScriptRunner + mcpScriptRunner)'
status: done
---

## Code Review

- [x] cranky-code-reviewer run on the diff
- [x] No critical findings
- [x] 3 significant findings addressed (nil-mutator rejection, stale doc refs, 7-vs-5 follow-up filed)
- [x] 3 minor findings addressed (doc accuracy, request.Mutator nil-rejection doc, manager-as-mutator test)
- [x] 2 leverage findings: 1 addressed (compile-time assertion), 1 deferred to TKT-IF37
- [x] Tests pass under `-race`
- [x] `just ci` green

## Disposition

See IMPL-CWDK for the full table.

**Headline wins from the review:**

- **Nil-mutator rejection.** First draft would have let a nil Mutator slip through `Run` and nil-deref inside gopher-lua. Now `LuaScriptRunner.Run` returns a typed error if a non-empty action is dispatched without a mutator. Test pins it.
- **Manager-as-Mutator pin.** Added `TestCreate_PassesManagerAsMutator` so any future refactor that drops `req.Mutator = m` fails loudly instead of silently breaking scripts that mutate.
- **Compile-time assertions.** `*Manager` now has compile-time `EntityManager` and `autocascade.Mutator` assertions — drift surfaces on Manager, not at distant call sites.
- **Follow-up filed.** TKT-IF37 captures the lua.WriteDeps narrowing work (5-method consumer interface in lua) so the 7-method Mutator can shrink in a focused PR.

**Type move (precursor commit):** mechanical, no smell — cranky verified no
stragglers.
