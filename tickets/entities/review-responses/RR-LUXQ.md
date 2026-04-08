---
id: RR-LUXQ
type: review-response
title: Lua sandbox concurrency invariants for ai.chat are unstated
finding: 'Plan asserts http.Client is safe (true) but never addresses the gopher-lua concurrency model. *lua.LState is NOT safe for concurrent use. If anyone calls ai.chat from a coroutine resumed from a different goroutine, data races. Fix: state the explicit invariant in design and in the top-of-file comment of internal/lua/ai.go: ''one ai.chat call per LState at a time; clients are safe to share across runtimes because the only shared state is http.Client''. Verify test coverage of the single-LState single-goroutine path.'
severity: significant
resolution: 'Top-of-file comment in internal/lua/ai.go now explicitly documents the LState concurrency invariant: ''ai.chat assumes single-threaded LState use. gopher-lua *lua.LState is NOT safe for concurrent goroutine use. ai.Provider implementations must be safe to share across runtimes.'' OpenAICompatProvider is safe because http.Client is. Plan''s risk table also lists LState concurrent use as a documented invariant.'
status: addressed
---
