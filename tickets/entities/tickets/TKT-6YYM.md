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
written by EntityManager.

## In scope

- New `internal/audit` package with an `Audit` interface and a JSONL filesystem implementation.
- `Audit` collaborator added to `WriteDeps`.
- EntityManager calls `Audit.Record` on every successful create/update/delete (entities **and** relations).
- Record shape: `{timestamp, op, entity_type, entity_id, actor, triggered_by, summary}`. Planning open question: include `outcome` field now (always `success`) for forward-compat with policy-deny logging.
- Actor: best-effort string today — `$USER` env → `git config user.email` → `"system"`. Becomes structured when Principal lands.
- `triggered_by`: populated for engine-initiated writes (automation, scheduler); empty for direct user actions.
- File layout: `.rela/audit/YYYY-MM-DD.jsonl`, append-only, daily rotation.
- Constructors reject nil collaborators per project rules.

## Out of scope (subsequent phases)

- Structured Principal type — actor stays a string for now.
- Write-policy hook (the EntityManager dispatch this ticket establishes will be reused).
- UI rendering of audit entries (raw file is the interface).
- CLI helpers (`rela audit tail`, etc.) — `tail -f` works.
- Read-side audit (no logging of reads).

## Why now

First concrete step in the multi-phase plan toward multi-user support. Useful
single-user immediately (forensics for scheduler/automation/MCP changes) and
de-risks the EntityManager dispatch infrastructure that write-policy and
principal-aware audit will both extend.
