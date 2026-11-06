---
id: RR-LSN9D3H
type: review-response
title: Wire does not carry MetaClosed
finding: computeRelationAffordances emits only the keys in Fields, so the UI shows an undeclared meta key as editable and the save returns 403.
severity: minor
reason: Needs a v1 wire field and a frontend change to RelationCards. The server refuses the write with a 403 that names the rule. Declared keys are already marked read-only because a closed verdict lists them all.
status: deferred
---
