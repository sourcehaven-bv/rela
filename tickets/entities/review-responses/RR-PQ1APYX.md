---
id: RR-PQ1APYX
type: review-response
title: EntityDetail grows further
finding: EntityDetail.vue is 4127 lines against a max-lines warning at 500.
severity: minor
resolution: Placement logic goes in a pure helper (channelPlacement.ts); EntityDetail only renders one extra block in the sections loop and filters the bottom block. EntityPageTabs.vue moved with the tab design to TKT-B75I7LH.
status: addressed
---
