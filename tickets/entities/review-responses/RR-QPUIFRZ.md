---
id: RR-QPUIFRZ
type: review-response
title: 'settle: one failing branch blocks the others and its error lands on another step'
finding: settle returned on the first branch error, so a broken branch kept a healthy one from settling, its error was reported to the person completing another branch, and backoff raised failures on untried branches.
severity: significant
resolution: settle settles the stepping branch first, backs off a failing branch alone (backoffBranch) and goes on with the others; Change ignores failures of other branches. A create give-back no longer resets the failure count. TestFork_JoinWaitsForPendingBranch checks it.
status: addressed
---
