---
id: RR-7SLP6
type: review-response
title: memoize sketch silently discards extra return values from fn
finding: The sketch uses `ls.PCall(0, 1, nil)` then `ls.Get(-1)`. Lua functions routinely return `(value, err)` pairs — users will write `rela.cache.memoize("k", function() return ai.complete(prompt) end)` and get back only the result, losing the `err` table in the `ai.*` convention deviation path. This is exactly the pattern AI calls produce, so silent truncation is a landmine.
severity: critical
resolution: 'Addressed in AC 11: `memoize` calls `fn` with `PCall(0, lua.MultRet, nil)`, captures all return values via stack-delta accounting (matching `RunActionString` at runtime.go:291), and re-emits all values on cache hit. Stored as `[]interface{}`. Test added for multi-return round-trip in AC 17.'
status: addressed
---
