---
id: RR-DM4HZR8
type: review-response
title: Rows with an empty title are re-read on every tick
finding: A title property set to the empty string gave search_title '' so the row stayed in the unset index and every heal tick read it again.
severity: minor
resolution: An empty title falls back to the id (nullif) in the trigger and the heal pass so a computed value is never empty. TestSearchTitle_WritesKeepItCurrent covers it.
status: addressed
---
