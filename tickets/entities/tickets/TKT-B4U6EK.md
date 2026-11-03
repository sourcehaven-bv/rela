---
id: TKT-B4U6EK
type: ticket
title: 'Schema lookups: and lookup() for table-driven values'
kind: enhancement
priority: medium
status: backlog
---

## Description

Out of scope for the G3 dashboard work. A risk matrix often derives a level from
two inputs (likelihood x impact) through a table the organisation defines. Add a
schema `lookups:` block that declares such tables and a `lookup(name\, a\, b)`
predicate function that reads them, so computed properties and conditions can
use the table instead of a long chain of conditions. Load-time checks: every
cell filled\, keys are declared enum values.
