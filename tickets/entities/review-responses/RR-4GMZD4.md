---
id: RR-4GMZD4
type: review-response
title: No reconnect/backoff test, no goroutine-leak check; reconnect logs only Debug
finding: 'cranky #4 + #5: the reconnect path (the whole point of ''feed survives a DB blip'') has ZERO test coverage — all tests are happy-path. Drive it with SELECT pg_terminate_backend(pid) on the listener''s backend, then assert a subsequent write still propagates. Also: no goleak.VerifyNone anywhere — for a feature headlined by a long-lived goroutine + dedicated connection, not asserting the goroutine exits on Close() risks a slow connection leak. Plus reconnect (listener.go:258-275) loops forever logging only at Debug (off in prod) — a permanently-down feed emits nothing after the initial open.go Warn; log at Warn after N failures / elapsed time so ''feed down 5min'' surfaces.'
severity: significant
resolution: 'Fixed: (1) TestListenerReconnects kills the listener''s backend via pg_terminate_backend(pid WHERE query LIKE ''LISTEN %'') and asserts a subsequent write still propagates (proves reconnect + re-LISTEN + catch-up). (2) Added go.uber.org/goleak; TestListenerGoroutineExitsOnClose opens a store, exercises the listener, Close()s, and goleak.VerifyNone asserts no leaked listener goroutine (pgx/puddle health-check goroutines ignored by name). (3) reconnect now escalates Debug->Warn after 5 consecutive failures (and logs Warn on recovery), with a comment on the fixed 2s backoff choice, so a persistently-down feed is visible in default log configs.'
status: addressed
---

## Resolution

(1) Add a reconnect test: terminate the listener's backend via
pg_terminate_backend (find pid via pg_stat_activity), then assert a later write
still propagates (proves reconnect + re-LISTEN + catch-up). (2) Add goleak
(VerifyNone / TestMain) to assert no listener goroutine leaks after Close. (3)
reconnect logs at Warn after N consecutive failures (escalate from Debug) so a
permanently-down feed is visible; add jitter/cap comment to the 2s backoff.
