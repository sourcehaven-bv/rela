---
id: RR-I49VIP
type: review-response
title: Corpus runtime against the CI package timeout
finding: About 120 s per run serially; under -race in CI the dataentry package may approach the 10 min default.
severity: minor
resolution: Recording runs in parallel; ForbidWebhookHeader takes a lock with a unit test. TestAnalyzeProperties_StopsScanningAtCap seeds 500 rows instead of 5000. The package race run takes 4m27s wall at load ~60, under the 10 min timeout.
status: addressed
---
