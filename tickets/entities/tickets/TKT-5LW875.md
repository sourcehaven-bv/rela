---
id: TKT-5LW875
type: ticket
title: 'Cardinality analysis: batch relation counts and count visible edges only'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Cardinality analysis (`internal/schema/cardinality.go`,
`internal/dataentry/analyze.go`) runs one `CountRelations` query per subject,
and since BUG-95W7MV one per face row for content-scoped outgoing bounds. The
counts are raw, so the "has N" message can reveal the number of hidden
neighbours to a principal who can read the subject (an existence oracle), which
conflicts with the gate-before-fold rule.

## Approach

Load edges with one `RelationQuery.EntityIDs` query per (relation, direction),
gate them with `Resolver.EndpointsReadable` on the gated paths, and count per
`(From, FromFace)` in memory. Pin with a `storetest.Counting` budget test at 10
and 50 rows. Decide whether the CLI (operator trust) keeps raw counts.

Raised by the BUG-95W7MV review (cranky #2, security minor).
