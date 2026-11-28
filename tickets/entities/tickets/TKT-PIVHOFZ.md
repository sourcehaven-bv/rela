---
id: TKT-PIVHOFZ
type: ticket
title: Push id relation filters down to the store
kind: enhancement
priority: medium
effort: s
started: "2026-11-28"
completed: "2026-11-28"
status: done
---

## Description

Follow-up to TKT-ZKPA1E. PR #2672 matches relation filters on the target id in
Go. This ticket answers an id filter in the store when the hop can be gated
there, so a filtered list pages in the database.

- Resolve the value once through the read gate. A readable id of a type the
relation reaches is the target; anything else stays a title.
- A lookup or gate fault fails the request instead of widening `ne`.
- Push the id filter into `store.GraphQuery` through `GateTraversal`; keep the
Go path for content-scoped relations, faced targets not read whole, and
unsupported traversals.

The work was built and reviewed in #2676 (code review, CISO approval) on an
older base and is ported onto develop here.
