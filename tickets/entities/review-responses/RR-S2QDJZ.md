---
id: RR-S2QDJZ
type: review-response
title: Compressed-row claim about empty busy cells is wrong
finding: 'Step 10 says an empty busy cell takes no space because isEmpty is true, and AC 7 relies on isEmpty. That holds for compact="stack" (RlTableRow.vue:546). EntityList always uses compact="compress" (EntityList.vue:1514), and there an empty primary cell keeps its place on purpose (RlTableRow.vue:611-623). So on a phone or beside a panel every row keeps an empty busy slot with padding. isEmpty only affects the stack layout here. Fix: correct the text. Decide whether a mostly empty busy column should hold space in compress, and pin that choice in the RlTable story.'
severity: minor
resolution: 'ADDRESSED step 10 and AC 7 corrected: in compress an empty busy cell keeps its slot by choice to avoid jitter, isEmpty only matters to the stacked layout; the BusyColumn story shows the empty slot in compress'
status: addressed
---
