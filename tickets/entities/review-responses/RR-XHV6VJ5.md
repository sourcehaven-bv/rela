---
id: RR-XHV6VJ5
type: review-response
title: Use goleak instead of counting goroutines
finding: A count only fails when leaks outnumber goroutines that exit during the run; goleak compares stacks and lists the leaked ones.
severity: minor
resolution: goroutines_test.go now snapshots with goleak.IgnoreCurrent after the warm-up cycle and calls goleak.VerifyNone. Confirmed it still fails on the old pin with the robfig/cron stack.
status: addressed
---
