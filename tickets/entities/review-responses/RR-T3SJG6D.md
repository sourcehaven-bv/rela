---
id: RR-T3SJG6D
type: review-response
title: Dry-run counts differ from a real run
finding: Relations to rows created in the same dry run were counted deferred.
severity: minor
resolution: A dry run tracks would-be links so such relations count as created; docs say a dry-run delete may become a detach. TestRun_DryRunCountsRelationsOfNewRows.
status: addressed
---
