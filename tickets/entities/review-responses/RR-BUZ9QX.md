---
id: RR-BUZ9QX
type: review-response
title: Windows lockFile checked only ERROR_LOCK_VIOLATION, so errLockHeld was dead code there
finding: 'LockFileEx is an overlapped-I/O call: a contended lock surfaces as ERROR_LOCK_VIOLATION on some paths and ERROR_IO_PENDING on others. Only the first was handled, so the second fell through to a generic error. It still failed closed (Open refused, and acquireProcessLock wraps every error in the ''another process is using...'' message, so the text was right by luck) — but ''lock held'' became indistinguishable from ''the lock syscall is broken'', and errLockHeld was effectively dead code on Windows. I could not test this platform directly, which is why it was flagged for scrutiny.'
severity: significant
resolution: Both codes now map to errLockHeld. Also added a comment recording that the fresh zero-value Overlapped is safe ONLY because the call is synchronous and immediate-fail — a future blocking or retrying path needs a properly-initialized Overlapped with an event handle, which is exactly the change someone would make without noticing. Verified GOOS=windows cross-compiles. The flags and the max-range lock were already correct.
status: addressed
---
