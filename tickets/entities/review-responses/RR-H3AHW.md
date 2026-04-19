---
id: RR-H3AHW
type: review-response
title: NewWriter with nil EntityManager/Meta panics at runtime instead of failing at construction
finding: 'internal/lua/runtime.go:165 — NewWriter accepts any WriteDeps including zero. Tests (date_test, markdown_test, scheduler_test) construct WriteDeps{} on purpose. If a test grew a mutation call, failure is a Go nil-deref, not a clean error. The reader path is strictly better than before (binding absent), but writer with nil manager is strictly worse (nil panic vs. typed error). Fix: in newRuntime, panic at construction with a clear message when allowWrites && deps.EntityManager == nil, and also when deps.Meta == nil. Fail loud at construction, not at a random Lua call moment.'
severity: significant
resolution: Added construction-time panic in newRuntime when allowWrites && deps.EntityManager == nil. Silent nil-deref at mutation call replaced by loud panic at lua.NewWriter. date_test, flow_test, markdown_test — none of which actually need writer semantics — switched to NewReader. script/executor_test.go stubEntityManager added to satisfy the guard in tests that intentionally use writer. New TestNewWriter_PanicsOnNilEntityManager covers the guard.
status: addressed
---
