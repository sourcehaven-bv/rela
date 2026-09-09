---
id: RR-E6VJI5
type: review-response
title: Test hygiene on the new fsstore tests
finding: Missing t.Parallel() and a discarded Close error next to a checked one
severity: nit
resolution: Close errors now checked via defer func(){ require.NoError(t; s2.Close()) }() matching the setup. t.Parallel deliberately NOT added - no test in recovery_test.go uses it; the file is serial and my tests match it.
status: addressed
---

Both new fsstore tests omit `t.Parallel()` while the manager test has it (ignore
if the surrounding file is deliberately serial).

`defer s2.Close()` discards its error a few lines after `require.NoError(t,
s1.Close())` — inconsistent, and a Close error on a store just aborted
mid-delete is exactly the thing worth knowing about.
