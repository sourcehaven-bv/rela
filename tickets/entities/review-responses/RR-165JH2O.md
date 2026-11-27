---
id: RR-165JH2O
type: review-response
title: entity_views stripped at config load
finding: The views-by-entity-type migration deleted the whole entity_views block, so mentioned_by_hide_id (and duplicate and mentioned_by_panel) never loaded.
severity: significant
resolution: The migration now strips only detail_view from each entry and drops an entry or the block only when empty. Verified in the browser.
status: addressed
---
