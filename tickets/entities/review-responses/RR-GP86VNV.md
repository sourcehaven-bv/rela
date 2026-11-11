---
id: RR-GP86VNV
type: review-response
title: E2E test does not prove only one tab notifies
finding: expect.poll stops at the first match, so a second notification from the other tab 200 ms later would pass; the title text was hardcoded English and expectShown was unused.
severity: significant
resolution: After the first notification bob posts a second mention; the holder must reach two notifications and the other tab must still have none. Titles are matched on the author, not English text. expectShown replaced by expectCount.
status: addressed
---
