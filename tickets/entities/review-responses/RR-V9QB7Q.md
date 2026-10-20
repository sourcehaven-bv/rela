---
id: RR-V9QB7Q
type: review-response
title: Cursor details in useChannel
finding: §3 `loadOlder` uses `before=<first seq>`. Use the page's `next_before` (the documented cursor). Also state explicitly that a write answer never advances the held rev (merging a post with rev R and then setting held=R would skip other writers' revs below R).
severity: nit
resolution: loadOlder uses the page's next_before, and a write answer never advances the held rev (R27).
status: addressed
---
