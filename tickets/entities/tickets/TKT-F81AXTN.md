---
id: TKT-F81AXTN
type: ticket
title: Check scoped relations on copy and conversion writes
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

TKT-5GK80Q9 enforces `within:` on relation writes through the manager. Entity
copy and variant conversion write relations through their own paths, which do
not run the scope check yet. Run the scope check (and its clears) on those paths
too, with tests on every backend.
