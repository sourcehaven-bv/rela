---
id: RR-HOFBBH
type: review-response
title: Fragile jq filter with head
finding: TITLE spliced into a double-quoted jq string piped to head -n 1.
severity: nit
resolution: Filter now selects [..][0].number // empty without head; verified against the repo (returns 1986).
status: addressed
---
