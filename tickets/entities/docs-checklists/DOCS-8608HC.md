---
id: DOCS-8608HC
type: docs-checklist
title: 'Docs: fixed scheduler identity + acl.yaml scheduler grant migration'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on new exported symbols — `principal.UserScheduler`, `migration.FileTypeACL`, `migration.SchedulerPrincipal`, `migration.EnsureMapping`, `migration.SetMapNode`, `ACLSchedulerGrantMigration`
- [x] Non-obvious decisions explained at the point of the code

Notably: `EnsureMapping`'s godoc states *why* it exists (null scalar vs empty
mapping, and the `InsertMapKeyAfter` no-op that made it a silent footgun), since
that is the trap the next migration author would otherwise step in.
`SchedulerPrincipal` records why the literal is duplicated rather than imported
(arch-lint restricts `internal/migration` to `[storage]`).

## Project Documentation

- [x] User-facing docs updated
- [x] Generated docs regenerated from source entities

`docs-project/entities/guides/GUIDE-scheduled-tasks.md` (SOURCE — `docs/*.md`
are generated and carry a "do not edit directly" banner):

1. The `run_as` section now names the actual default identity (`system:scheduler`,
fixed, independent of the OS account) instead of the vague "the scheduler's
system user".
2. New subsection "If your project has an `acl.yaml`, grant the scheduler" —
states the silent-failure symptom, shows the grant, points at `rela migrate`,
and tells operators to narrow it. Explicitly says policy-less projects need do
nothing.
3. Audit-log section notes that `principal.user` is now the task identity and
that pre-upgrade records carry the OS account.

Regenerated via `just docs`; the pre-push docs-freshness gate passes.

- [x] ~~README~~ (N/A: no new top-level capability)
- [x] ~~CLAUDE.md~~ (N/A: no new architectural rule — this follows the existing migration pattern)

## External Documentation

- [x] ~~API docs~~ (N/A: no HTTP API surface changed)
- [x] Migration notes — the migration's `Description()` is the operator-visible
string and names the symptom, not just the mechanic: "Grant the scheduler
identity read access in acl.yaml (without it, scheduled tasks silently read
nothing)"
- [x] Breaking change documented — audit attribution for scheduled writes
changes from the OS account to `system:scheduler`
