---
id: RR-GUCLTN
type: review-response
title: Over-bound test also passes on old code
finding: TestPatchRelations_DisallowedTypeOverBoundIs422 is a regression guard, not evidence for the audit fix.
severity: nit
resolution: Comment states it guards that the manager path keeps the bound check.
status: addressed
---
