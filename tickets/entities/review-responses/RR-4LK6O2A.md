---
id: RR-4LK6O2A
type: review-response
title: Pinned search_path breaks writes after a schema rename
finding: SET search_path FROM CURRENT on the trigger function pinned the schema name at migration time. After the documented ALTER SCHEMA RENAME every entity write would fail or read another schema's title map.
severity: critical
resolution: The trigger reads search_title_props through TG_TABLE_SCHEMA as 0021 does. The heal pass computes the same expression in Go. TestSearchTitle_WritesKeepItCurrent renames the schema and writes; TestSearchTitle_HealAgreesWithTrigger holds the two expressions together.
status: addressed
---
