---
id: RR-ABDV43
type: review-response
title: Relation affordance gate is not injected
finding: entitymanager cannot see the dataentry relation gate (relationSources, when:), so the recipe could create an edge the user cannot create by hand and offers would drift from commit.
severity: significant
resolution: 'Plan: consumer-side TaskItemEdgeGate in entitymanager like CopyEdgeGate; NewTaskItemConverter rejects nil; wired over FieldGated(em) in wireTypeChanges; same gate in Offers and Convert.'
status: addressed
---
