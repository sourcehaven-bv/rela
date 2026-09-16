---
id: 'memlock-abandoned-drop-polled'
type: 'automated-measure'
title: 'Test: the abandoned-acquire cleanup drops its map entry, asserted by polling rather than by one read'
description: 'Guards against BUG-KSEQ3T. The test still pins the real property (an abandoned waiter must not leak its refcount) but now polls for up to 5 seconds instead of reading the entry count once, because the cleanup goroutine publishes that count after the unlock the test synchronised on. Mutation-tested rather than merely re-run: with l.drop(key, e) deleted from the abandoned path the test still fails, so the polling did not trade detection for stability. 30 of 30 runs pass under -race after the change against 8 failures in 20 before.'
kind: 'test'
location: 'internal/lock/memlock_test.go:TestMemoryLocker_AbandonedAcquireReleases'
status: 'active'
---
