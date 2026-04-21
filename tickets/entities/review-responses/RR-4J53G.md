---
id: RR-4J53G
type: review-response
title: Time injection missing - TTL and LRU tests will be slow/flaky
finding: The test plan uses `time.Sleep(1.1s)` for TTL expiry and `time.Sleep(1*time.Microsecond)` for LRU tie-breaking. `time.Sleep` at microsecond granularity is not reliable under CI load, and 1.1s x several TTL tests is real wall time.
severity: minor
resolution: 'Addressed in AC 17: ''TTL expiry (time source injected - no time.Sleep)''. The Cache gets a `now func() time.Time` field defaulted to `time.Now`, overridden in tests via a `WithNow` option or direct field assignment. This is a new convention for the codebase worth flagging in the commit message.'
status: addressed
---
