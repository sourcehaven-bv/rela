---
id: RR-BWHEV
type: review-response
title: Process-wide scope claim overstates lifetime
finding: '''Process-wide'' is accurate per command but users will misread it as durable across invocations. Additionally, internal/cli/flow.go creates workspace.Discover(scriptDir, script.NewEngine()) on every flow invocation — flow steps may not share a cache. Rephrase docs to ''in-memory, shared across all Lua runtimes in a single process; not persisted. Each new rela invocation starts with an empty cache.'''
severity: minor
resolution: 'Rewrote Cache docstring in cache.go, CLAUDE.md paragraph, and docs/lua-scripting.md Scope section. Now explicit: ''in-memory, shared across all Lua runtimes in a single process, not persisted, each new rela invocation starts with an empty cache''. Flow caching note deferred (realistic audit later).'
status: addressed
---
