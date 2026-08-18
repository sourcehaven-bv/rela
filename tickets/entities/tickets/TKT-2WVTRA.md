---
id: TKT-2WVTRA
type: ticket
title: Struct-derived config projector with construct-coverage conformance test
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Problem

Build the real projector: `*metamodel.Metamodel` + `dataentryconfig.Config` →
entities + relations, per the meta-schema (TKT-IB5C8S).

Read-only and in-memory. No store, no write path, no storage decision.

## Why struct-derived, not hand-mapped

The spike hand-mapped fields and got three wrong:

| Guessed | Actual | Caught by |
| --- | --- | --- |
| `Kanban.GroupBy` | `Kanban.ColumnProperty` | compiler |
| `NavigationEntry.View` | does not exist | compiler |
| nav label in `.Label` | groups use `.Group` + `.Items` | **only as bogus analyze errors** |

The third produced 4 phantom "required field missing" errors and 6 phantom
orphans that looked like real findings. A hand-maintained projection **rots
silently** as config structs evolve — and §5.7 showed that a lossy projection
buries its real findings under artifacts (1 real finding vs ~116 artifacts in
the spike run).

So: derive from the Go structs (reflection or codegen), and ship a **conformance
test asserting every config construct projects** — the test is the anti-drift
mechanism, and is as important as the projector.

## Scope

- `Project(meta, cfg) ([]entity.Entity, []entity.Relation, error)` or similar
- Struct-derived field mapping
- Conformance test over every construct in `metamodel.Metamodel` and `dataentryconfig.Config`
- Cross-file edges (`renders-type`, `binds-property`, `binds-relation`, `groups-by-type`)
- Dangling-reference reporting (spike found zero in `tickets/` — the config is internally consistent, and that check is itself a lint)

## Out of scope

Write-back, storage, the SPA. Fidelity gaps are the next ticket.

## Open questions (resolve when work starts)

- **Reflection at runtime, or codegen at build time?** Codegen is debuggable and greppable; reflection is less code. Codegen probably wins given arch-lint and the plimsoll load lines.
- **Where does this package live?** It must import both `internal/metamodel` and `internal/dataentryconfig`; check `just arch-lint` allows that direction, or whether it needs to sit above both.
- **How does the conformance test detect a NEW struct field** that the projector ignores? Reflection over the struct compared against a declared-covered list is the obvious approach, but needs an exemption mechanism for genuinely non-projectable fields.

## Context

Spike: `.ignored/schemaspike/cmd/main.go` + `dataentry.go` (~600 lines,
throwaway). Findings §5.5, §5.7.
