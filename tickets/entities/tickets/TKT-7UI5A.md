---
id: TKT-7UI5A
type: ticket
title: Remove view system, replace with Lua equivalents
kind: refactor
priority: medium
effort: m
status: done
---

Remove the views.yaml declarative graph traversal system (internal/views/, CLI
commands, MCP tools). Lua scripting already covers the core query/traversal
capabilities. Provide example Lua scripts for view deps and view affected
patterns to ensure no functionality is lost.
