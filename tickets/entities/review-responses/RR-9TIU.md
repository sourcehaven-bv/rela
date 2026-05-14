---
id: RR-9TIU
type: review-response
title: 'Architect #6: cascadeHost.CreateEntity discards warnings'
finding: cascadeHost.CreateEntity returns (*Entity, error) per autocascade.Host contract — soft-validation warnings on cascade-created entities are dropped.
severity: significant
reason: Filed as TKT-MSR8. Fixing requires widening autocascade.Outcome with Warnings []autocascade.Warning and changing Host.CreateEntity signature — out of TKT-IU2S scope. cascadeHost.CreateEntity has a doc comment referencing the limitation.
status: deferred
---
