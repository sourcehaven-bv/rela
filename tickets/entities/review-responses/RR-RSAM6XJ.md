---
id: RR-RSAM6XJ
type: review-response
title: 'Design: one place computes the due-time hint'
finding: next_due_at must be the minimum over branches on every save; backoff overwrote it and advance handled one token.
severity: minor
resolution: 'Plan updated: save computes the run-level hint from all tokens; backoff sets the delay on the failing tokens only; advance handles every due token.'
status: addressed
---
