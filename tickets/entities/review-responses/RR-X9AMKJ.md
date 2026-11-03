---
id: RR-X9AMKJ
type: review-response
title: Package-var ceilings mutated while two conformance runs run in parallel
finding: 'internal/channels/channelstest/waiting.go:167-177 assigns channels.MaxWaitingRows, and :185 reads MaxWaitingAnchors. TestConformance_Disk and TestConformance_MemFS (internal/channels/filechannels/file_test.go:79,96) both call t.Parallel(), and both run RunAll, which ends in waitingTests. One run''s lowered ceiling can make the other''s ListWaiting fail with ErrTooManyRows, and the accesses are unsynchronized: a data race under CI''s -race. It did not reproduce in two local -race runs, because the MemFS run usually finishes first, but nothing orders it. Fix: make the ceiling a store option or a WaitingQuery field (MaxRows) set by realtime, so tests lower it per call without touching globals. At minimum, serialize the two runs.'
severity: minor
resolution: Bounds are constants; WaitingQuery.MaxRows and a per-service bounds field set per-call ceilings so tests no longer mutate globals
status: addressed
---
