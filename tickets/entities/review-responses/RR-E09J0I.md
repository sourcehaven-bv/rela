---
id: RR-E09J0I
type: review-response
title: useAutoSave.ts is over 1000 lines
finding: Base tracking is spread across functions; a small MergeBases object would hold the rule in one place.
severity: nit
reason: The review fixes put the base rule behind holdsProp/holdsContent and the Conflicts record. A further split of useAutoSave.ts is a refactor outside this ticket.
status: wont-fix
---
