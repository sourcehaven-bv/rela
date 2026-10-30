---
id: TKT-FUQL97
type: ticket
title: Prose-check registry and engine with analyze prose
kind: enhancement
priority: medium
effort: l
started: "2026-10-30"
completed: "2026-10-30"
status: done
description: 'Add the checkers: registry to the metamodel and a check engine that runs registered prose checkers through cmdexec; expose findings through rela analyze prose.'
---

## Problem

First of three PRs for prose checks (FEAT-K5VQS9). The design, its design review
and the decisions are in TKT-EKY2TV, which keeps the editor endpoint, the
Milkdown decorations and the MCP `prose` check.

## Scope

In:

- `checkers:` registry in the root `schema.yaml`: `command`, `format`
(`vale` | `rela`), `exit_codes`, `timeout`, `analyze`. Rejected in includes.
Locked in the Configure space.
- When checkers are declared, entity type names must match
`^[A-Za-z0-9][A-Za-z0-9_-]*$`.
- cmdexec: `PurposeCheck` with `RELA_SANDBOX_CHECK_READ_PATHS` and
`rela-server --sandbox-check-read-paths`; `RunWith` with a per-call input name,
accepted exit codes and a shorter timeout; the child runs in the run's temp dir
on every platform.
- `internal/check`: the engine, one finding shape, Vale and native parsers.
The body is written to `<type>/<id>.md`.
- `rela analyze prose [--type T]` and `rela analyze all --prose`.
- Docs: `docs/checkers.md`, the CLI reference, the sandbox table, the rules
file.

Out: the `_check` endpoint and editor (TKT-EKY2TV), MCP, batch mode, dismissing
findings (TKT-L0CACN).
