---
id: RR-CTX2
type: review-response
title: Authorization input read outside the serialization
severity: critical
status: addressed
finding: authorizeCascadeRelations resolved FromType, the value the ACL decision is made on, through the
  OUTER store handle. On pgstore that is the pool, so it read outside the very transaction the restructure
  exists to establish, and took a second connection while the first was held.
resolution: Threaded the tx view through authorizeCascadeRelations; the lookup now uses tx.GetEntity.
---
