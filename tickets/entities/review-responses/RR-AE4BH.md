---
id: RR-AE4BH
type: review-response
title: Stale Lua state across entities when rule errors
finding: 'Plan Decision 4 promised ''no semantic regression'' but only the happy path was verified. When entity 1 raises mid-script, the runtime carries half-built coroutines, partial print buffers, and module-locals into entity 2. Resetting only the entity global is not enough. Location: internal/validation/lua.go:133-163 (validateLuaWithRuntime).'
severity: significant
resolution: CheckRule now closes and rebuilds the per-rule runtime after each ScriptError so partial coroutines, leaked locals, and other half-built state from a failed iteration cannot be observed by subsequent entities. Two new lifecycle tests cover the leak-after-error and error-on-every-entity paths. Commit b03c17b.
status: addressed
---
