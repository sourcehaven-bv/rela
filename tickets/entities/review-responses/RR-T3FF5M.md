---
id: RR-T3FF5M
type: review-response
title: Entity schema inlined per operation
finding: Each response inlines buildEntitySchema; a components ref per type would shrink the spec and give generated clients one model per type.
severity: nit
reason: A spec-size and client-ergonomics improvement independent of this ticket's goal.
status: deferred
---
