---
id: RR-3D0CJC
type: review-response
title: 'Design: try-lock makes tagging fail intermittently'
finding: pg purge uses try-lock; a tag during a sweep tick would fail.
severity: significant
resolution: Blocking advisory lock bounded by lock_timeout/ctx; concurrent-move tests (R5).
status: addressed
---
