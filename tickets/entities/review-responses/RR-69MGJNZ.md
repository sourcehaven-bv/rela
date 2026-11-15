---
id: RR-69MGJNZ
type: review-response
title: _page_status ignores the world
finding: The walk reads rows in the page's world but /_page_status did not take ?world=, so a world-bound page was judged over default-world rows.
severity: significant
resolution: Admitted _page_status in worldCapablePath and in the world guard's scanned set. The SPA passes the world and keys the query on it. TestPageStatus_JudgesTheWorldsRows pins it.
status: addressed
---
