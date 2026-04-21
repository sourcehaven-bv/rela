---
id: RR-3IF9O
type: review-response
title: SetScriptPath is wrong shape of public API (round 2)
finding: SetScriptPath is an order-dependent state setter exposed on the Runtime. The architectural preferable is RunStringAs(name, code) or RunFileContent(path, []byte) so callers don't need to sequence SetScriptPath then RunString. Only one non-engine caller (MCP lua_run) needed it.
severity: minor
resolution: Added Runtime.RunFileContent(path, content, args) — same effects as RunFile (chunk name, rela.args, cache namespace) but takes already-read bytes for callers using os.OpenRoot. Updated MCP lua_run to use RunFileContent instead of SetScriptPath+RunString. SetScriptPath remains for inline-code callers (validation rules with pseudo-paths, script.Engine.execute with inline code) but its doc comment now explains the split vs RunFileContent.
status: addressed
---
