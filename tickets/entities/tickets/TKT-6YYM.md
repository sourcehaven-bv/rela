---
id: TKT-6YYM
type: ticket
title: 'Audit log: append-only JSONL of entity write operations'
kind: enhancement
priority: medium
effort: m
status: ready
---

## Summary

Add an append-only audit log for every entity and relation create/update/delete,
written by `entitymanager.Manager`.

## In scope

- New `internal/audit` package with an `Audit` interface and three backends (`Nop`, `Memory`, `Filesystem`).
- `Audit` added as a required collaborator on `entitymanager.Deps` (validated in `entitymanager.New`).
- `Manager` calls `Audit.Record` on every successful create/update/delete/rename (entities **and** relations).
- Record shape: `{time, op, entity_type, entity_id, actor, triggered_by, summary}`. No `outcome` field in v1 (every record is a success by construction; policy-deny logging is a future phase).
- Actor: best-effort string today — `$RELA_ACTOR` → `$USER` → `git config user.email` → `"system"`. Becomes structured when Principal lands.
- `triggered_by`: populated for engine-initiated writes (automation, scheduler) via `context.Context`; empty for direct user actions.
- File layout: `.rela/audit/YYYY-MM-DD.jsonl`, append-only, daily UTC rotation.
- Production wiring sites: `appbuild.Discover` (CLI / server / desktop) and `appbuild.NewForTest` (tests). `Audit` is constructed once at the wiring site and threaded into `entitymanager.New` via `Deps`.
- Tests get an ergonomic default: `appbuild.NewForTest` auto-populates `Audit: audit.Nop{}` unless a `WithTestAudit(...)` option overrides it.

## Out of scope (subsequent phases)

- Structured `Principal` type — actor stays a string for now.
- Write-policy hook (the Manager dispatch this ticket establishes will be reused).
- UI rendering of audit entries (raw file is the interface).
- CLI helpers (`rela audit tail`, etc.) — `tail -f` works.
- Read-side audit (no logging of reads).
- Log retention/cleanup policy — documented as an operator concern.

## Why now

First concrete step in the multi-phase plan toward multi-user support. Useful
single-user immediately (forensics for scheduler/automation/MCP changes) and
de-risks the `entitymanager.Manager` dispatch infrastructure that write-policy
and principal-aware audit will both extend.

## Refactor context (post-workspace decomposition)

This ticket was originally written against `internal/workspace.wsEntityManager`.
That surface no longer exists — `internal/workspace` was deleted in the arc
TKT-QTNX → IU2S → DS43 → UG3C → 64R3 / 2IAC. The write chokepoint is now
`entitymanager.Manager` (`internal/entitymanager/manager.go`); the wiring facade
is `appbuild.Services` (`internal/appbuild/`). Both already accommodate a new
required collaborator without churning call sites — `entitymanager.Deps` is
explicitly designed to grow (its doc comment mentions "audit, principal, policy
in subsequent tickets"), and `appbuild.NewForTest` is the single test fixture
that all `_test.go` files use, so adding an audit default lands in one place.
