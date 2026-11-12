---
id: RR-T77TZ2O
type: review-response
title: A job state can rest with no way out
finding: With only guarded done branches a success with no holding guard leaves the run stuck forever.
severity: significant
resolution: 'Plan: a job state''s last done branch must have no when:; the load refuses it otherwise.'
status: addressed
---
