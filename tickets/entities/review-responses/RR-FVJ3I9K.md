---
id: RR-FVJ3I9K
type: review-response
title: Soft-deleted entities still cost file reads per create
finding: SoftDelete().ListMarked loads entity meta from disk for every marked family on each create.
severity: minor
reason: Trash is small in practice and the seed creates none. The broader id-minting redesign for all backends is TKT-69TDK8.
status: deferred
---
