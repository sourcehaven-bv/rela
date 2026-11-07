---
id: RR-51RPUN2
type: review-response
title: IsManagedName hard-codes the namespace separator
finding: entity.IsManagedName uses "." instead of metamodel.NamespaceSeparator.
severity: nit
reason: entity sits below metamodel and cannot import it.
status: wont-fix
---
