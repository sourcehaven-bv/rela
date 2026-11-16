---
id: RR-MK94XMB
type: review-response
title: related ignored the face and repeated targets
finding: A faced ref listed the edges of every tail and a target linked from two tails appeared twice.
severity: significant
resolution: A ref with a face sets RelationQuery.FromFace; targets are deduplicated and sorted by id. TestRelatedReadsOneTail pins both.
status: addressed
---
