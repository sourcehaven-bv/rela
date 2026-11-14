---
id: RR-K9TZ9RL
type: review-response
title: Dashed prefix splits SQL and Go backends
finding: SequentialNumber trims a trailing dash from the prefix but the SQL builders used prefix + "-" raw, so HighestID("REQ-") differed between file and SQL backends.
severity: minor
resolution: buildHighestIDSQL trims the trailing dash on pg and sqlite; bounds tests and the storetest case now cover the prefix REQ-.
status: addressed
---
