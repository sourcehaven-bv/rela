---
id: IMPL-O1VDT
type: implementation-checklist
title: 'Implementation: Add cache API for Lua scripts (get/set + memoize, process-wide with per-script namespace)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (newCachedWriter helper)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

End-to-end smoke test with `rela script scripts/cache_smoke.lua` against a fresh
project at `/tmp/cachetest/`:

```
calls = 1
r1[1] = result    r2[2] = from fn
OK
```

Confirms:

- First memoize call invokes fn (calls==1)
- Second memoize call with same key does NOT invoke fn (calls stays 1)
- Multi-return values round-trip correctly (r1[1]="result", r2[2]="from fn")
- `rela.cache.set` / `rela.cache.get` round-trip (assertion passes)

All 24 cache unit tests pass (`go test ./internal/lua/ -run TestCache`). Full
project test suite passes (`go test ./...`).

## Quality

- [x] Code follows project patterns (check similar code — matches WithAIProvider/WithSecrets)
- [x] No security issues introduced (no disk I/O in v1; log fields are hashed; no raw key/path in errors)
- [x] No silent failures (unrepresentable values, inline context, bad options — all raise)
- [x] No debug code left behind

## Implementation summary

**Files added:**

- `internal/lua/cache.go` (~300 lines) — Cache type, bindings, validation
- `internal/lua/cache_test.go` (~400 lines) — 24 test cases

**Files modified:**

- `internal/lua/runtime.go` — `cache`/`scriptPath` fields; `WithCache` option;
`SetScriptPath` method; `registerCacheBindings` call; `RunFile` sets path
- `internal/script/executor.go` — Engine holds a Cache; passes via WithCache;
SetScriptPath in execute()
- `internal/workspace/workspace.go` — ScriptExecutor.LuaCache() method;
NopScriptExecutor returns nil
- `internal/workspace/services.go` — Workspace.LuaCache() accessor
- `internal/workspace/analysis.go` — validation wired with shared cache
- `internal/validation/validation.go` — Service.WithCache method
- `internal/validation/lua.go` — Reader runtime takes cache; pseudo script path
- `internal/cli/script.go` — Wires cache via lua.WithCache
- `internal/cli/flow.go` — Wires cache via lua.WithCache
- `internal/mcp/server.go` — Services.LuaCache in the interface
- `internal/mcp/tools_lua.go` — Wires cache; sets script path for lua_run
- `internal/dataentry/app.go` — App holds a long-lived script.Engine
- `internal/dataentry/actions.go` — Uses app-scoped engine (cache persists across requests)
- `.testcoverage.yml` — 80% floor for internal/lua
- `docs-project/entities/guides/GUIDE-lua-scripting.md` — Cache section + API
- `docs/lua-scripting.md` — regenerated
- `CLAUDE.md` — One-paragraph pointer

**AC verification map:**

- AC 1: `lua.Cache`, `NewCache`, `.get`/`.set`/`.delete` exported — cache.go
- AC 2: `lua.WithCache` option added — runtime.go
- AC 3: `TestCacheUnregisteredWithoutOption`, `TestCacheBehaviourWithNilCacheOption`
- AC 4: `TestCacheRunFileSetsScriptPath`
- AC 5: `TestCacheInInlineRaisesError`
- AC 6: `TestCacheNamespacedIsolation`
- AC 7: `TestCacheSetAndGetRoundTrip`, `TestCacheTTLExpiry`
- AC 8: `TestCacheSetNilDeletes`, `TestCacheTTLZeroNeverExpires`
- AC 9: `TestCacheSetRejectsLongKey`, `TestCacheSetAcceptsMaxKey`
- AC 10: `TestCacheSetRejectsFunction`, `TestCacheSetRejectsNestedFunction`
- AC 11: `TestCacheMemoizeHitSkipsFn`, `TestCacheMemoizeMultipleReturns`, `TestCacheMemoizeFnRaisesNotCached`
- AC 12: `TestCacheGetRejectsOptions`, `TestCacheSetRejectsUnknownOption`, `TestCacheMemoizeRejectsUnknownOption`, `TestCacheMemoizeBypass`
- AC 13: `TestCacheMemoizeConcurrentBothRun`
- AC 14: `TestCacheLRUEvictionAtCap`
- AC 15: `TestCacheLoggingNeverLeaksRawKey`, `TestCacheErrorMessagesDoNotLeakKey`
- AC 16: All entry points updated (see Files modified)
- AC 17: All test scenarios covered (see above)
- AC 18: `.testcoverage.yml` floor 80% — `just coverage-check` passes
- AC 19: `docs/lua-scripting.md` Cache section; CLAUDE.md pointer
