---
id: TKT-JIYY50O
type: ticket
title: Next-action count sources read every visible row to answer whether one exists
kind: enhancement
priority: low
status: backlog
---

## Description

A next-action `count:` source without `count_ungated` answers "does the caller
see any entity of this type?" through `countIsZero`, which reads every visible
row with `scopedSortedEntities` and checks the length. One visible row is enough
to answer.

The type default query scope applies there, and `scopeRequest` refuses a read
window together with a `Scope` (the scope filters after the store read). So the
fix is either a windowed read when the type has no default scope, or reading in
windows until one row passes the scope.

Found while working on TKT-CKLZMWB. The perf project has no count source, so the
scale run does not measure it.
