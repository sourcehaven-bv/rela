---
id: RR-R6YXKM
type: review-response
title: Far-future or corrupt next_retry wedges a task permanently and silently
finding: 'The retry gate is unbounded: if NextRetry[name] exists and now < retryAt, runDueTasks continues with no log line. A clock skew (VM snapshot resume, NTP step, bad RTC) or hand-edited state file writes a far-future retryAt that persists across restarts. Verified: with next_retry 100 years out, 0 executions over a simulated 30 days, no WARN or ERROR emitted. Violates the CLAUDE.md no-silent-failures rule. A pending retry can never legitimately exceed maxRetryDelay into the future, so it can be clamped on load with a WARN.'
severity: critical
resolution: 'Clamp added at the retry gate: a retryAt more than maxRetryDelay in the future is treated as now, with a WARN naming the task and the implausible time. Clamping at the gate rather than only on load also catches a clock jump mid-run. Covered by TestRunDueTasks_implausibleRetryTimeIsClamped and verified live: a seeded year-2126 next_retry produced the WARN and retried immediately instead of hanging.'
status: addressed
---
