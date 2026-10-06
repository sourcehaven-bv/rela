---
id: RR-IJU1J8
type: review-response
title: Holding the migration lock around the save deadlocks the runner
finding: Runner.Run acquires the non-reentrant lock itself (run.go:118-125, lock.go:19-22).
severity: critical
resolution: 'Plan changed: the save takes only its own mutex; the runner takes the lock; ErrLockHeld maps to 409 ''try again''.'
status: addressed
---
