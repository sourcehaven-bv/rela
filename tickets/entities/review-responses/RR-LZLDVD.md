---
id: RR-LZLDVD
type: review-response
title: Precomputed per-type block maps are unnecessary
finding: typesWithFields/typesWithOptions are not needed. Fold declared blocks first, then only for a dimension that ended up opted in on this row add the universe for block-less roles that hold the verb. Less work, and the historical opt-in and per-field option opt-in need no special case.
severity: minor
resolution: Dropped typesWithFields/typesWithOptions. The fold applies blocks first, then asks block-less roles for the verb only on dimensions a block closed on this row.
status: addressed
---
