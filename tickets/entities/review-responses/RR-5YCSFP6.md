---
id: RR-5YCSFP6
type: review-response
title: "A list condition makes nav items prime twice"
finding: "nav_items.go: when the list has a condition, listPage primes the whole population on a ctx it does not return, and navItemsFor primes the page again. Fixed cost, but untested."
severity: significant
resolution: "Kept the second prime at the handler level so the change stays independent of TKT-QM833U, which reworks the shared list path. The comment now says it is a second fixed-size batch. Added a nav-items budget case with a related() list condition under the field policy, pinned at 12 reads at both sizes (168 at 50 rows without the fix)."
status: addressed
---
