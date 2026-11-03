---
id: RR-ANA3JV
type: review-response
title: mcp.Deps plimsoll raise to 21 when the cheap split is named in the comment
finding: server.go:64 raises max-fields to 21 and says the next capability should group the Lua pair. CLAUDE.md says to prefer splitting over raising. Grouping LuaWriteDeps and LuaCache now costs about the same as the directive.
severity: nit
resolution: Grouped LuaWriteDeps and LuaCache into Deps.Lua (mcp.LuaDeps{WriteDeps, Cache}) and removed the //plimsoll:max-fields=21 directive. just plimsoll is clean.
status: addressed
---
