---
id: RR-BC4P60I
type: review-response
title: Paging and size of the gated list
finding: The list document can be large (RR-XGCZSM5). Paging before gating would leak counts.
severity: minor
resolution: Gate the whole list in batches, then page (per_page max 200). Counts come from the gated set.
status: addressed
---
