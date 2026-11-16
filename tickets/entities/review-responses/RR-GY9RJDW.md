---
id: RR-GY9RJDW
type: review-response
title: One bad record discards the whole batch
finding: ParseBatch rejects the whole reply when one record has a bad value, so one row wastes the run for all.
severity: significant
resolution: 'Plan: structural errors fail the run; a missing record or bad value becomes a failed entry with entity, title and reason.'
status: addressed
---
