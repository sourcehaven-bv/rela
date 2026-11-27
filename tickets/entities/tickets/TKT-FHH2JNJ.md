---
id: TKT-FHH2JNJ
type: ticket
title: 'Flows: fault-path tests for write events'
kind: chore
priority: low
effort: s
status: backlog
---

## Description

Split from TKT-GYXOTQC (RR-WF0IRHC). Add tests for the write-event paths that
need store fault injection:

- a revision conflict on the subject update;
- a rollback after the update hook ran (no run moved or started);
- a step write on a subject with a second open run;
- a nested-Tx update holding no roles (fail closed);
- the predicate depth bound reached by an update;
- a kept change taken through the job path.
