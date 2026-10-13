---
id: TKT-RQ6BSA
type: ticket
title: Extract the webhook find/create/then mapping core out of dataentry
kind: refactor
priority: medium
effort: m
status: ready
---

## Description

Move the declarative webhook mapping core (`find:` / `match:`,
`create_if_missing:`, `then:` with `set` / `append_section`, interpolation and
the conflict retry) out of `internal/dataentry` into its own package, so the
feed subscription runner (TKT-RUO1GY) can reuse it. No behaviour change:
existing webhook tests must pass unchanged. Second of three steps for
FEAT-A53JJF.
