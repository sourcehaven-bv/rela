---
id: RR-Q7C9Y
type: review-response
title: 'Combined Lua + content rule: content silently skipped on Lua failure'
finding: 'Pre-change, both Lua and content checks ran independently per entity. Post-change, when Lua fails (scriptErr != nil), the function returns early at line 251, skipping the content check. A rule with both lua: and content: now degrades partially when the Lua portion errors. Undocumented behavior change. Location: internal/validation/validation.go:218-263 (checkEntityAgainstRule).'
severity: minor
resolution: checkEntityAgainstRule no longer returns early when Lua errors. ScriptErrors are collected, then the content check runs independently and any content violation is appended. Lua-violations still short-circuit content (matching pre-change semantics). New test TestLuaValidation_LuaErrorDoesNotSuppressContentCheck verifies both surfaces are reported. Commit 74a1ab5.
status: addressed
---
