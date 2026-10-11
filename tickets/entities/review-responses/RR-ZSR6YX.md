---
id: RR-ZSR6YX
type: review-response
title: Hop count not shared across nodes or old binaries
finding: tokenMu is process-local, and during a rolling deploy an old node ignores the token's count.
severity: minor
reason: Transient or needs two nodes writing one key in the same instant; documented on tokenMu. Stopping a chain later than 8 hops is bounded by maxAutomationJobRuns and the retry policy.
status: wont-fix
---
