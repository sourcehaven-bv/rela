---
id: RR-GP805D
type: review-response
title: Rename resets per-channel rate limits
finding: EntityRenamed drops the anchor's buckets, so a caller who may rename gets fresh per-channel tokens.
severity: minor
resolution: EntityRenamed moves the buckets to the new id, keeping the emptier bucket on a clash.
status: addressed
---
