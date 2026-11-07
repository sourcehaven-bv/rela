---
id: RR-46QOIBH
type: review-response
title: Goroutine tests must stay sequential
finding: A t.Parallel on these tests would let other tests' goroutines count as leaks.
severity: nit
resolution: Helper doc says tests using it must not call t.Parallel.
status: addressed
---
