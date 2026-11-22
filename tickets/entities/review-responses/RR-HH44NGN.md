---
id: RR-HH44NGN
type: review-response
title: 'An invalid UTF-8 job error could block the run'
finding: 'The 512-byte cap ran before JSON storage widened invalid bytes to U+FFFD, so a stored error could exceed MaxJobError and fail CheckState.'
severity: significant
resolution: 'jobErrorText makes the text valid UTF-8 before the cap; TestJobErrorText round-trips it through JSON.'
status: addressed
---
