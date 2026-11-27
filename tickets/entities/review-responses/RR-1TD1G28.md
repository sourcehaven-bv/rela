---
id: RR-1TD1G28
type: review-response
title: Sort param is not a reader-sort signal
finding: Grouped lists always send their group sort, so gating manual order on an absent sort would never apply it there.
severity: significant
resolution: The SPA sends manual_order=1 while the reader has not sorted; the server applies positions only then.
status: addressed
---
