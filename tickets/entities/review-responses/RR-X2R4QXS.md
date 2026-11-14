---
id: RR-X2R4QXS
type: review-response
title: Actions cache can evict the weekly baseline
finding: Caches unused for 7 days are evicted, and the job runs every 7 days.
severity: significant
resolution: 'Addressed: the baseline is an artifact (scale-run-baseline, 90 days) of the newest successful run on the default branch, fetched with gh run download; actions/cache is no longer used.'
status: addressed
---
