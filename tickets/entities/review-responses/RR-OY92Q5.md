---
id: RR-OY92Q5
type: review-response
title: Unfinished-migration prompt lost on reload
finding: The config-changed event reloaded the tab after an incomplete save, and nothing showed the unmigrated state afterwards.
severity: significant
resolution: A sessionStorage flag survives the reload and reopens the drawer until a retry succeeds.
status: addressed
---
