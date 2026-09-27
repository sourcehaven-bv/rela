---
id: RR-5YZKF8
type: review-response
title: DirectedRelation zero Direction means both directions
finding: store.DirectionBoth is the zero value (store.go:504); pgstore treats non-Outgoing as incoming while naive ListRelations matches both, so backends would disagree.
severity: significant
resolution: 'Plan: field is `Incoming bool` (as acl.TraversalHop), so there is no both-directions value.'
status: addressed
---
