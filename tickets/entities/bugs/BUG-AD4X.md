---
id: BUG-AD4X
type: bug
title: Ctrl+C does not interrupt in-flight Lua operations
description: The Lua runtime's applyTimeout() rooted its LState context at context.Background, so caller cancellation (e.g. Ctrl+C via cobra) did not propagate. Any rela script, rela flow, or MCP lua_eval/lua_run call would run to completion (or hit the 30s internal timeout) regardless of SIGINT.
priority: high
why1: applyTimeout() always used context.Background() as the parent, ignoring any caller context.
why2: The runtime API had no way for callers to pass their context in (only WithTimeout).
why3: The original design treated the Lua timeout as the only cancellation mechanism, since scripts were pure/CPU-bound and Ctrl+C was not considered.
why4: Scripts have since grown network-calling bindings that can block for up to 30s, making the lack of caller-driven cancellation user-visible.
why5: No systemic hook in CLI startup ever wired a signal-aware context down to embedded runtimes.
prevention: Added lua.WithContext option plus signal.NotifyContext at cli.Execute so cancellation flows from SIGINT/SIGTERM into the Lua LState.
status: done
---
