---
id: RR-GX3LBQ
type: review-response
title: memstore-only tests miss the real risks
finding: Type drift, case collisions, filename ids and skipped folders only occur with fsstore as source.
severity: significant
resolution: 'Plan updated: backendcopy tests use fsstore as source over a fixture with those cases; the sqlite integration test reuses it.'
status: addressed
---
