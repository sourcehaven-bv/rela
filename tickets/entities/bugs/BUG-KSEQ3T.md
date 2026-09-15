---
id: 'BUG-KSEQ3T'
type: 'bug'
title: 'TestMemoryLocker_AbandonedAcquireReleases reads the entry count before the cleanup goroutine has dropped its refcount'
description: 'The test releases the original holder, waits for a fresh Acquire to succeed, then immediately asserts MemoryLockerEntries == 0. Those two events are not ordered. On the abandoned path MemoryLocker.Acquire spawns a detached goroutine that does e.mu.Unlock() and THEN l.drop(key, e); the unlock is what lets the fresh Acquire through, so the acquire can complete while drop has not yet run. Measured at 8 failures in 20 separate runs on pristine develop with no dependency change, so the postgres e2e and Go suites both inherit a roughly 40 percent chance of a red build from a locker that is behaving correctly. The production code is correct; only the test''s assumption about ordering is wrong.'
priority: 'medium'
why1: 'The test asserted on the map entry count immediately after a successful Acquire, treating that acquire as proof the cleanup goroutine had finished.'
why2: 'It is not proof. The cleanup unlocks the mutex before dropping the refcount, so the acquire it unblocks races the map write that follows it.'
why3: 'The unlock-then-drop order is deliberate and correct (it hands the lock on as early as possible), but the test was written against the intent of the cleanup rather than its actual instruction order.'
why4: 'Sibling assertions in the same file read the entry count after a fully synchronous release, where the count IS settled. The same one-line assertion was reused on the asynchronous path, where it is not.'
why5: 'A test asserting on state that another goroutine publishes needs a synchronisation point or a poll, and nothing in review or CI distinguishes the two cases. The failure reads as a real leak, so it invites investigation of correct production code.'
prevention: 'The assertion now polls for up to 5 seconds for the entry count to reach zero. Mutation-tested rather than merely re-run: deleting the l.drop(key, e) call from the abandoned path still fails the test, so polling did not weaken what it detects. 30 out of 30 runs pass under -race after the change, against 8 failures in 20 before. Generalizable lesson: when a test observes state written by a goroutine it did not join, poll for the property with a timeout, and confirm the polling version still fails a genuinely broken implementation before trusting it.'
status: 'done'
---
