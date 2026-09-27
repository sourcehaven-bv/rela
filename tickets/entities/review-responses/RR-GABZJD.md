---
id: RR-GABZJD
type: review-response
title: Entity-inheritance closure seeded from the whole type on a one-page MatchingIDs
finding: The entity closure CTE seeded from every entity of the type before the page's id list filtered anything, so page cost grew with the type (115 ms for 50 ids over 5000 items at depth 5).
severity: significant
resolution: graphSource now takes the page's id subquery and seeds the closure from it, with unary + on the type so the primary key drives. TestMatchingIDsEntityClosureSeedsFromThePage asserts the plan uses no type index and scans no table. The same shape in pgstore is part of BUG-LCHDSR.
status: addressed
---
