---
id: TKT-UO8UIGO
type: ticket
title: 'rela test v1 (3/4): rela test command, rela.test Lua module, seeded fakers and principals'
kind: enhancement
priority: medium
effort: xl
status: ready
---

## Description

Part 3 of TKT-0MCLX9S. The design is in PLAN-G8YMBDB; scope, AC1-9 and AC11-16
are listed there.

- **New package `internal/apptest`:** runner, world (NewInMemory + fsimport fixture load + part-2 seams), the Lua module, seeded schema-aware fakers (gofakeit behind `t.fake`), and the report.
- **Principals:** `t.as(user)` takes roles from the real policy; `t.principal{roles}` creates a user and a per-case policy clone passed with WithACL. Add `principal.ToolRelaTest`.
- **Access expectations:** `accessExpectation` moves to `internal/aclmap`.
- **Command:** `internal/cli/test.go` is outside requiresProject. It supports `--run`, `--seed` and `-o json`, and refuses config-in-rela.db projects with a clear message.
