---
id: RR-LN3HGE
type: review-response
title: no-principal case asserts too little
finding: 'internal/realtime/busy_test.go:376-384 seeds no channel and does not assert that ListWaiting was not called, unlike the table cases above it. Fix: post the claim first and assert `h.store.waitingLists.Load() == 0`.'
severity: nit
resolution: The no-principal case posts the claim first and asserts ListWaiting was not called.
status: addressed
---
