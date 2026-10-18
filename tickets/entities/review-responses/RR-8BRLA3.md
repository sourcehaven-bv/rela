---
id: RR-8BRLA3
type: review-response
title: Idle subscriber change set grows without bound
finding: Local.mark accumulates every changed channel key for a stalled subscriber.
severity: minor
resolution: The change set is capped at 1024; past that Take returns a resync flag.
status: addressed
---
