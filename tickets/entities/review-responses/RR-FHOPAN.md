---
id: RR-FHOPAN
type: review-response
title: 'Review test-gap list: two real gaps closed, four claims rebutted by existing tests'
finding: 'The review listed six test gaps: (a) dry-run never acquires, (b) concurrent stale-break race, (c) Tick maps ErrLockHeld to Skipped, (d) gate skips under contention, (e) deadPID helper skips on stripped containers, (f) LockFor selection untested.'
severity: significant
resolution: '(b) was real and is closed by TestFSLock_ConcurrentStaleBreakSingleWinner (plus the TOCTOU fix it pins, RR-0EPN1X); the ledger-scope variant of (d) is closed by TestGate_ContendedAdoptionSkipsLedgerToo. (a), (c), (d-marker), (f) were already covered before the review — TestRunner_DryRunNeverTouchesLock, TestGC_ApplySkipsWhenLockHeld (asserts Skipped + nil error + data intact + lock-free dry-run tick), TestGate_AdoptionSkipsOnContention, and TestLockFor_Selection (three-case table incl. the store-capability path) all exist in lock_test.go; the reviewer stalled mid-run and evidently did not finish reading the test file. (e) accepted: the skip only triggers on hosts without /usr/bin/true or /bin/true, which no CI runner in this repo matches.'
status: addressed
---
