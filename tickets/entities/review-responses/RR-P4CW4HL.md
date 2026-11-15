---
id: RR-P4CW4HL
type: review-response
title: BodyWatch comment cites GraphQueryHeaders as counting fallback bodies
finding: The new comment said fallback bodies are counted as GraphQueryHeaders does; GraphQueryHeaders drains b.Store.GraphQuery and skips the decorator so it does not. The older comment makes the same claim and ListEntityHeaders has the same gap.
severity: nit
reason: Dropped the false comparison from the new comment. The pre-existing ListEntityHeaders and GraphQueryHeaders gap predates this ticket and changing it can move other budget tests; filed as TKT-5OWNZAU.
status: deferred
---
