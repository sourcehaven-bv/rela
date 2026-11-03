---
id: RR-1MPRGI
type: review-response
title: Store conformance does not isolate a rev-only lifecycle change or cross-process races
finding: channelstest.go:850 changes state and rev together, so a backend that compared only state would pass; the rev-only reopen+close case exists only against the realtime fake (lifecycle_test.go:300). Add a store-level reopen-then-close case at an unchanged state, and a concurrent Delete vs Append case through `Harness.Second` for pg.
severity: minor
resolution: 'DeleteCompareAndSet now reopens and closes again, so the state matches the read and only the rev moved, and asserts ErrConflict. New conformance test DeleteRacesAppend races Delete against Append through Harness.Second for 20 rounds: either the delete is refused and both messages remain, or the delete wins and only the later append remains. It passes on file, sqlite and postgres (verbose run shows PASS).'
status: addressed
---
