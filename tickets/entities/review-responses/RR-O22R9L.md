---
id: RR-O22R9L
type: review-response
title: DirectionNoSide conflated bad inputs with a genuine wrong-side; api_v1 fallback laundered them all to outgoing
finding: 'InferDirection returned DirectionNoSide for five distinct situations (nil metamodel, empty entityType, empty relation, unknown relation, genuine wrong-side) — three of which are not about sides at all. resolveFormRelations then funnelled every non-Resolved outcome into DirectionOutgoing via a bare else, with a comment asserting "both already reported by validation". That assertion is load-bearing and unenforced: s.Meta == nil would silently ship outgoing for every binding in the app, reintroducing the exact default the ticket removed, in the place furthest from the author''s eyes.'
severity: significant
resolution: 'Added DirectionUnknown as a distinct outcome (inference could not run: no metamodel/entity type/unknown relation) versus DirectionNoSide (a statement about the relation). resolveFormRelations now switches on the outcome explicitly, and logs a warning for DirectionAmbiguous — which should be unreachable because ValidateConfig rejects it at load, so reaching it means the gate was bypassed and that must be visible rather than silently resolved.'
status: addressed
---
