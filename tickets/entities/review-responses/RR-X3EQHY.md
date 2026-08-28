---
id: RR-X3EQHY
type: review-response
title: Lua migration step had no execution budget — runaway script hangs the run
finding: The sandboxed LState never got a context (internal/lua sets SetContext for exactly this); ctx was only checked between entities, so `while true do end` hung `rela migrate data --apply` unrecoverably mid-file with the marker un-advanced.
severity: significant
resolution: 'ls.SetContext(ctx) binds the VM to the run context (commit bddc13f3): a runaway script is now interrupted by Ctrl-C/ctx cancellation and surfaces as a step error before the marker moves; the idempotent-steps recovery story applies. Pinned by TestLuaStep_RunawayScriptInterruptedByContext (infinite loop + 200ms ctx timeout).'
status: addressed
---
