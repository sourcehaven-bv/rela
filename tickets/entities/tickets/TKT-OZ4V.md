---
id: TKT-OZ4V
type: ticket
title: Extract shared workspace bootstrap helpers (production + tests)
kind: refactor
priority: medium
effort: s
status: wont-fix
---

## Summary

Production entry points (`cmd/rela-server`, `cmd/rela-desktop`,
`internal/cli/root.go`) and ~10 test files each construct `workspace.Workspace`
from scratch with similar boilerplate. Adding any new required collaborator
currently churns ~20 call sites. Extract shared bootstrap helpers so future
additions land additively.

## Status: wont-fix — superseded

This ticket was reframed during planning. The original premise — *build helpers
around `Workspace` so future collaborators thread cleanly* — is rowing in the
wrong direction: per CLAUDE.md, `internal/workspace` is a transitional shim and
new code should not extend it. Building shared helpers around the legacy
aggregate is *more* work to throw away than just decomposing it now.

The decomposition plan replaces this ticket with a series of focused refactors.
See:

- TKT-6OMC — Extract `automation.Runner` with consumer-side `Host` interface
- TKT-QTNX — Define `entitymanager.Manager` (real implementation, not adapter)
- TKT-KWAX — Migrate MCP server to wire its own services
- TKT-2IAC — Migrate scheduler to wire its own services
- TKT-9JEI — Migrate dataentry server to wire its own services
- TKT-0SP1 — Migrate CLI commands to scoped consumer-side interfaces
- TKT-Y0JU — Narrow `lua.WriteDeps.EntityManager` to an `EntityMutator` interface
- TKT-64R3 — Delete `internal/workspace` once consumers have migrated

The audit-log ticket (TKT-6YYM) waits for the EntityManager extraction
(TKT-QTNX) so audit lands as an explicit field on `entitymanager.Deps` directly,
with no Workspace involvement.
