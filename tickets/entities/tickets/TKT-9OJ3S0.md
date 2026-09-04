---
id: TKT-9OJ3S0
type: ticket
title: Observer backfill for state rows
kind: enhancement
priority: medium
effort: s
status: backlog
---

Design doc §12.7. Observers are not invoked for entities already on disk
(`fsstore.go:69-73`); `backfillBleve` exists for entities. State rows need the
same backfill story so search indexes existing states after an upgrade or after
faces are enabled on a type.

**Enumeration surface (architect decision, 2026-08-19):** consume
`EntityQuery.AllStates bool` from TKT-DOFYR1 PR-A — the raw storage-truth
enumeration flag (NOT world resolution; worlds are a separate compiled predicate
from Step 2). Backfill walks `AllStates: true` to index every state row as it
exists in storage.
