---
id: RR-1SFK52
type: review-response
title: Agree-test is a flat loop without t.Run and omits degraded, never-written and many rows
finding: lifecycle_test.go:345-385 builds about 180 harnesses inside one test body, so a failure names no subtest and later cases never run. Wrap each combination in t.Run and add the degraded and `calls` (many) rows, so AC7 covers the whole decision function in one table.
severity: nit
resolution: 'Rewrote it as a table of 108 parallel t.Run subtests: room in each state with the condition true or false, the room never written, and a calls channel open or closed; each crossed with 3 callers, 2 ages and degraded on or off. Each subtest checks all four operations.'
status: addressed
---
