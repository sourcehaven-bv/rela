---
id: RR-WCH0E6
type: review-response
title: 'direction: on a symmetric relation yields different counts for the two endpoints'
finding: 'The plan left this as an open to-do (''confirm direction semantics are coherent, or reject'') inside a checklist marked done, and my first reading resolved it wrongly toward ''accept and treat both directions as equivalent''. A symmetric relation is stored as ONE row; no reciprocal edge is ever written — Symmetric appears nowhere in internal/store/** or internal/entitymanager/**, and Manager.CreateRelation makes exactly one store call. Symmetry is a read-time presentation convention only (dataentry/relations_direction.go:49-55, default_view.go:92-95 skips the incoming pass to avoid double-counting the same edges). So for symmetric type T with stored edge A->T->B, a query from B finds nothing: A and B get different counts for the same relationship, decided by write order.'
severity: significant
resolution: 'Verified: grep for Symmetric in internal/store and internal/entitymanager returns nothing, confirming no reciprocal row. My earlier ''treat both directions as equivalent'' note is superseded. Plan and ticket now specify: direction: on a symmetric: true relation is a LOAD ERROR, in the validateValidationRelations style, with an acceptance criterion. Self-referencing relations are separately noted as legitimately two-directional (a self-loop matches both directions and counts once under each).'
status: addressed
---
