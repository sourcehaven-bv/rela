---
id: RR-XAB5E
type: review-response
title: Scheduler test stubs executeTaskFunc so new LuaWriteDeps call path is untested
finding: internal/scheduler/scheduler_test.go:43, 120-124 — tests override s.executeTaskFunc so doExecuteTask (which calls ws.LuaWriteDeps() and engine.ExecuteFile) never runs. Coverage ratchet won't notice since the added LuaWriteDeps() method body is trivial. Not a regression — old tests did the same — but since we now require the WorkspaceProvider interface to include LuaWriteDeps, at least one test should exercise that path end-to-end.
severity: minor
resolution: 'Added TestDoExecuteTask_PullsLuaWriteDeps in scheduler_test.go. mockWorkspace now counts LuaWriteDeps() calls; the new test invokes doExecuteTask (no executeTaskFunc stub) with a real script.NewEngine(), asserts LuaWriteDeps() was called exactly once. Script itself is intentionally missing so ExecuteFile returns an error — the test verifies the deps-pull path, not successful script execution. Scheduler coverage: 71.5% → 77.1%.'
status: addressed
---
