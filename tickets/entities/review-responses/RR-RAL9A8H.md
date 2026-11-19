---
id: RR-RAL9A8H
type: review-response
title: No test that a guard error stays guard
finding: The err == nil gate keeps blocked from overwriting guard; no assertion pins it.
severity: minor
reason: Change returns Blocked false with ErrGuard by construction (the GuardFailed case returns before the default case); the existing guard tests keep the guard answer.
status: wont-fix
---
