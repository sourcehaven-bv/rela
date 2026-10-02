---
id: RR-QMS200
type: review-response
title: Multi-value enum cells show only the first value
finding: tableCellFor compared cell.widget to 'multiselect' but the server sends 'multi-select'; a list enum got values[0] and rendered one badge.
severity: critical
resolution: 'Routing now uses the schema PropertyDef (densePropertyRoutingHint); a list property passes the full values array. Test: list enum renders three badges.'
status: addressed
---
