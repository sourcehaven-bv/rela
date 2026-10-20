---
id: RR-XXFUBG
type: review-response
title: Timing-based SSE tests should use testing/synctest
finding: The test plan uses expiries of "a few hundred milliseconds with generous read deadlines". AC2 asserts that something does not happen within a time interval. On a loaded CI runner with -race, such a test either flakes or makes the suite slow. The module uses Go 1.26, so testing/synctest is available. Run handleSSE inside a synctest bubble, where the expiry timer, ping ticker and flush timer all advance on a fake clock. Use an in-process ResponseWriter that implements Flush and SetWriteDeadline, not a real httptest server, because synctest does not treat a goroutine blocked on network I/O as durably blocked. AC1, AC2 and the edge case of an expiry while a flush is armed then become deterministic. If synctest proves awkward with the full router, at least compute the stub's expiry when the stub is called, not during test setup.
severity: minor
resolution: 'Plan updated: the expiry tests use testing/synctest with an in-process ResponseWriter.'
status: addressed
---
