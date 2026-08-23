---
id: RR-0EPN1X
type: review-response
title: 'fs stale-break TOCTOU: two processes could both break and both acquire'
finding: Between one process judging the lock file stale (dead pid) and removing it, another process could have already broken it and re-created a FRESH lock — the first process's os.Remove then deleted the new holder's file and both acquired. Additionally, release's unconditional os.Remove could delete a file created by a different holder after a manual lock removal.
severity: significant
resolution: 'Two mechanisms (commit d514cc8e): (1) breaking now requires winning a break-mutex file (O_EXCL `.lock.break`; abandoned break markers age out after 30s) and re-verifying staleness UNDER it, so only one breaker exists and a fresh lock is never removed; (2) release removes the lock file only while it still contains this acquisition''s exact payload (removeIfOurs), so a stale release can never delete a new holder''s file. Pinned by TestFSLock_ConcurrentStaleBreakSingleWinner (20-round two-goroutine race on a seeded dead-pid file, exactly one winner, -race) and TestFSLock_StaleReleaseDoesNotRemoveNewHoldersFile.'
status: addressed
---
