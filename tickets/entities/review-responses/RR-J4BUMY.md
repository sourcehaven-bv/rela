---
id: RR-J4BUMY
type: review-response
title: kvpiles mutex does not span reassembly
finding: A per-instance mutex does not exclude a successor Services built over the same KV during reassembly.
severity: minor
resolution: 'Plan: ForReassembly carries the predecessor''s piles service (like attachLocker), so one instance and one mutex serve both.'
status: addressed
---
