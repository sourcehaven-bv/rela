---
id: TKT-64R3
type: ticket
title: Delete internal/workspace once all consumers have migrated
kind: refactor
priority: low
effort: s
status: backlog
---

## Summary

After the MCP, scheduler, dataentry, and CLI migration tickets land,
`internal/workspace` has no remaining production consumers. Delete the package.

## In scope

- Remove `internal/workspace/` entirely.
- Remove `workspace` references from `.go-arch-lint.yml`.
- Update CLAUDE.md: remove the "Don't extend `internal/workspace`" entry under "Don't do this" since the package is gone.
- Move the small parts of Workspace that don't live elsewhere yet (e.g., per-project config + state KV bundling) into a focused successor service or inline into the wiring sites.

## Out of scope

- Any new functionality. Pure deletion.

## Depends on

- All four migration tickets (MCP, scheduler, dataentry, CLI) landed and merged.
- All test files migrated off `workspace.NewForTest` / `workspace.WithFS` / `workspace.WithTestStore` (some test sites may need to migrate to focused-service stubs as part of this ticket if they weren't done in their own migration).

## Why

This is the goal-state ticket. Workspace as a god-object is removed. New
collaborators (audit, principal, policy) thread through focused services without
the legacy aggregate in their way.

## Risks

- Hidden consumers — internal Workspace methods that turned out to be relied on by tests or by code paths not surveyed during migration. Run a comprehensive grep before deleting.
- Test coverage may drop if Workspace's tests aren't ported to the focused services. Verify each migration ticket carries its tests with it.
