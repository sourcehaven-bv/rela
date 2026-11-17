---
id: RR-WOJL98T
type: review-response
title: 'Design: prune cutoff with clock skew between processes'
finding: Prune deletes by seq up to the last row whose at is before the cutoff. With clock skew between rela-server processes a few rows appended after the cutoff by wall clock can go with them.
severity: minor
resolution: Prune now deletes only up to the row before the first row appended at or after the cutoff, so skew cannot pull newer rows in. Pinned by PruneStopsAtFirstYoungRow.
reason: Deleting by seq is what keeps the remaining rows gapless and verifiable. The effect is bounded by the clock skew, and the retention floor of 365 days dwarfs it. Documented in Prune's godoc.
status: addressed
---
