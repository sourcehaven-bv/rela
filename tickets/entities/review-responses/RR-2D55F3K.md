---
id: RR-2D55F3K
type: review-response
title: Test plan misses several cases
finding: 'Missing: reload with one follower; remote rename or delete; remote mark read; multi-process appends; listener failed at Open; LISTEN before gap; resync limit; CLI post hint before exit; post latency on postgres.'
severity: significant
resolution: All added under Test Scenarios (AC3, 5a-5d, 6, 7, 8a and a postgres benchmark).
status: addressed
---
