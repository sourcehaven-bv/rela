---
id: RR-XVG7URE
type: review-response
title: Backfill misread a property named moved_session
finding: The text test matched a props key named moved_session.
severity: minor
resolution: The text test is a prefilter; sqlite reads json_extract of the top-level member and postgres a jsonb helper. Migration tests cover the case.
status: addressed
---
