---
id: RR-MIY1IH
type: review-response
title: Wall-clock sleep in a unit test
finding: 'internal/mcp/tools_session_test.go:205 sleeps 1.1s with time.Sleep to move the clock. entered_at rendered in RFC3339Nano, so any elapsed time would have shown the regression; the sleep only adds runtime to every run. Fix: inject the fixture clock and advance it, as TestSessionView_UnstoredNoEnteredAt does with h.clock.Add.'
severity: nit
resolution: The test uses an injected fake clock and advances it instead of sleeping
status: addressed
---
