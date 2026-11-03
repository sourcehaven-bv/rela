---
id: RR-R2J9KD
type: review-response
title: Flattening tests pin a wrapper production no longer calls
finding: Flattening tests pin a wrapper production no longer calls
severity: significant
resolution: The webhookPayload.interpolate wrapper is gone. The dataentry flattening tests now call ingest.Interpolate with payload.Resolve. TestRun_FlattensEveryTemplatedField in ingest covers create properties, create content, seeded match values and set steps.
status: addressed
---
