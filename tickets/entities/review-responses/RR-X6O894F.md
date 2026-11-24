---
id: RR-X6O894F
type: review-response
title: 'Code: faces pooled in scope pairs'
finding: 'scopePair and the edge queries carry no face, so a faced source pools edges across faces.'
severity: minor
reason: 'The loader refuses content-scoped scoped relations. An identity-scoped edge belongs to the entity, not to a face, so pooling per entity is the intended meaning.'
status: wont-fix
---
