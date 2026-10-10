---
id: RR-WTX98H
type: review-response
title: Probe can write a stale verdict after reset
finding: A probe in flight across a reset wrote its old answer.
severity: minor
resolution: Generation counter; reset bumps it and late answers are dropped. Test added.
status: addressed
---
