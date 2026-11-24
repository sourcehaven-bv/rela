---
id: RR-7JRZN5F
type: review-response
title: 'Security: copy transaction did not run the scope check'
finding: 'applyCopyEdges creates and removes relations after CheckOwningEdge without the scope check.'
severity: significant
resolution: 'applyCopy runs checkCopyScopes (refuse-only, like automations) on the planned adds and removes before any write. A copy carries only content-scoped relations and a scoped relation cannot be content-scoped, so the guard cannot fire today; TestParse_CopyCannotCarryScopedRelation pins that.'
status: addressed
---
