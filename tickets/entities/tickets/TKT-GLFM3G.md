---
id: TKT-GLFM3G
type: ticket
title: Fail closed when no read gate is on the request context
kind: enhancement
priority: medium
effort: s
status: backlog
description: Under a Declarative policy a request without a gate on ctx falls back to nopReadGate (permits all, readgate.go:224). Fail closed instead; fix tests that call handlers directly.
---

## Description

Follow-up from the TKT-WFS778 design (question 3). Under a Declarative policy, a
request whose context carries no read gate falls back to `nopReadGate`, which
permits every read (internal/dataentry/readgate.go:224). A wiring mistake
therefore opens reads instead of denying them.

## Scope

- Return an error (or deny) when the policy is Declarative and no gate is present.
- Update tests that call handlers without the middleware.
- After TKT-WFS778 this logic lives in internal/access.
