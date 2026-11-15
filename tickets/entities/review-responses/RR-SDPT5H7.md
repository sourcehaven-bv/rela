---
id: RR-SDPT5H7
type: review-response
title: Search ordering test assertion was vacuous
finding: TestACLList_SearchAfterACLOrdering kept the last MatchingFaces call, which the row gate also makes, so the order check could not fail\; its name described the old order.
severity: significant
resolution: Renamed to TestACLList_SearchHitsAreGatedByIDRead\; the store logs MatchingFaces ids and the test finds the call over exactly the hits. The data assertion (a hidden hit is dropped) stays.
status: addressed
---
