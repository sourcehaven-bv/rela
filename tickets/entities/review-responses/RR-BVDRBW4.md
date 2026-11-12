---
id: RR-BVDRBW4
type: review-response
title: A permanent error at the deadline is stored as timeout
finding: HandleAttempt checked the deadline before ErrPermanent, so an idempotent job retried a permanent error returned as the deadline passed.
severity: minor
resolution: ErrPermanent is checked first. TestJob_PermanentErrorAtTheDeadlineIsPermanent.
status: addressed
---
