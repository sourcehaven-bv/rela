---
id: RR-N2R3I3
type: review-response
title: Schema reloaded per page
finding: filterEntities(h.schema(), ...) reloaded the schema snapshot on every page.
severity: minor
resolution: collectBacklinks captures the schema once before the paging loop.
status: addressed
---
