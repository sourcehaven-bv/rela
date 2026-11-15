---
id: RR-LPQD349
type: review-response
title: Rows restored before the title map keep a wrong search_title
finding: pg_dump loads entities before search_title_props so the trigger computes every restored row from an empty map. The heal pass only visited queued types and empty rows so those rows stayed wrong.
severity: significant
resolution: The first heal pass after each open checks every row and rewrites only the rows that differ. TestSearchTitle_HealAgreesWithTrigger writes a row before its type is in the map and checks that the full pass fixes it without rewriting correct rows. The migration comment no longer claims restore is covered by the trigger alone.
status: addressed
---
