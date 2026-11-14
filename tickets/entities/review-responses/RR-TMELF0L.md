---
id: RR-TMELF0L
type: review-response
title: Dead GraphQueryer arms in HolderLookup
finding: store.Store embeds GraphQueryer, so the not-GraphQueryer arms in holders.go and headerquery.go are dead.
severity: nit
reason: Harmless defensive arms outside this ticket's scope.
status: wont-fix
---
