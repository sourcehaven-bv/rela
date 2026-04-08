---
id: RR-GLT3
type: review-response
title: 'F19: Runtime AI calls don''t propagate the caller''s cancellation context (pre-existing)'
finding: applyTimeout built the Lua-state context from context.Background(), never from a parent. So Ctrl+C to rela script wouldn't interrupt an in-flight AI HTTP call — only the 30-second HTTP client timeout would. Pre-existing issue but became user-visible once Lua scripts could make 30-second network calls.
severity: minor
resolution: 'Resolved upstream by PR #329 (fix(lua): propagate caller cancellation into runtime), which landed in develop as 0f76c4a before TKT-YBKB''s rebase. The Lua runtime now accepts a parent context via lua.WithContext(ctx), and rela script / rela flow / MCP lua_eval+lua_run all wire cmd.Context() through a signal-aware context built via signal.NotifyContext at cli.Execute. Ctrl+C to rela script now interrupts both pure-CPU Lua loops AND in-flight ai.chat calls.'
status: addressed
---
