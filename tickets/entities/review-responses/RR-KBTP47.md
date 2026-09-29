---
id: RR-KBTP47
type: review-response
title: refOf/entityRef and resolutionRuleAt wrapper still present
finding: The design deletes dataentry.refOf/entityRef and moves provenance onto WorldScope.RuleAt; the dataentry wrapper and entityRef remain.
severity: nit
reason: 'Intended: PR 1 excludes call-site migration. entityRef/refOf go in PR 3 and the resolutionRuleAt wire wrapper is revisited when the resolver shares RuleAt in PR 2.'
status: deferred
---
