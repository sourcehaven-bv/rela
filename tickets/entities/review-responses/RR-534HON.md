---
id: RR-534HON
type: review-response
title: tryStep duplicates step
finding: 'live_busy_test.go:138-155 copies liveStepper.step (live_inbox_test.go:90-110) except for the failure path. Fix: implement step as for range n { if !s.tryStep() { tb.Fatal(...) } } so the two cannot drift.'
severity: nit
resolution: 'Fixed: tryStep moved next to step in live_inbox_test.go, and step now loops over tryStep.'
status: addressed
---
