---
id: TKT-41YLNDP
type: ticket
title: Run automations and cascades inside the batch transaction
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Run on-create automations and cascades inside the batch transaction, so their
writes are part of the atomic unit (RES-QKREXZZ ticket 6). Needs a view-bound
`cascadeHost` and Mutator, buffered cascade audit, and `jobs.WithDeferral` at
the batch boundary. Lua then runs under the deployment-wide write lock, so it
needs a policy against external I/O.
