---
id: RR-7YI3DH2
type: review-response
title: Retention was never applied
finding: Service.Expire had no caller, so rows of a type with retention were kept for ever.
severity: significant
resolution: startRecordRetention runs Expire as a recurring job with a fixed IdempotencyKey, like the soft-delete GC. TestRecordRetention_DropsExpiredRows.
status: addressed
---
