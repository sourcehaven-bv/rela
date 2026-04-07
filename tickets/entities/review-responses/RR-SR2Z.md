---
id: RR-SR2Z
type: review-response
title: compareOrdered swallows unknown operators silently
finding: 'Returns false for unknown operators with no panic or error, and applyV1Filters'' default case includes entities for unknown operators. Inconsistent: one excludes, one includes. Pick a philosophy.'
severity: minor
reason: compareOrdered returning false for unknown operators is intentional defensive behavior. The applyV1Filters switch only calls it with valid operators (the case is exhaustive), so this branch is unreachable in practice. Aligning the default behavior of applyV1Filters' switch is a separate cleanup.
status: deferred
---
