---
id: RR-Y6O02Y
type: review-response
title: Among by peer id cannot tell two faces of one source apart
finding: On the incoming side two edges from one source can sit in the same list (draft and published tails). OrderPosition.Among holds peer ids, so a hidden tail of a visible source would be planned and written. Make Among a list of relation keys.
severity: significant
resolution: OrderPosition.Among is []RelationKey; orderSiblingFilter matches by Identity(), so two faces of one source are two entries.
status: addressed
---
