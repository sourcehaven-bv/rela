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

TKT-5GK80Q9 enforces `within:` on relation writes through the manager. Variant
conversion writes relations through its own path, which does not run the scope
check yet. Run the scope check (and its clears) there too, with tests on every
backend. Entity copy already runs a refuse-only check (checkCopyScopes).
