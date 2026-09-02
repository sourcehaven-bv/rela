---
id: TKT-NS3XPE
type: ticket
title: 'output.Writer: delete dead/internal/single-caller surface — plimsoll directive deleted (23 → 14)'
kind: refactor
priority: medium
effort: s
status: done
---

Sub-ticket of [[TKT-N0IKN9]]. Unlike `cli.CLI` (surveyed NO-GO: a declarative
kong grammar table with zero field consumers — a documented structural
exception, not a coupling surface), `output.Writer` has 24 real consumer
packages and 9 of its 23 exported methods are demonstrably not earned width:

## What

1. **Delete dead `WriteRelations`** — zero production callers (tests only);
the table machinery it exercises stays covered via `writeEntitiesTable`.
2. **Unexport `WriteSeparator` / `WriteFooterSummary`** — called only from
inside output.go.
3. **Extract the 6 schema-JSON methods** (`WriteSchemaOverview/Entities/
Relations/Types/EntityDetail/RelationDetail`) plus the 3 schema interfaces
(`SchemaMetamodel`, `SchemaEntityDef`, `SchemaRelationDef`) out of `output` —
single caller is `internal/cli/schema.go`; four are one-line JSON wrappers that
never branch on Format, i.e. schema serialization that accreted onto the nearest
big type.
4. **Delete `//plimsoll:max-exported-methods=23`** — the type lands at 14,
under the default 20. Directive deleted, not ratcheted (the TKT-45QYI outcome).
5. Fix the stale count drift in `internal/cli/kong.go`'s directive comment
(says 38; directive says 46; actual is 46) and reword it as a documented
structural exception per the survey verdict.

## Done when

plimsoll passes with the output directive gone; full suite green (output_test
schema tests re-pointed, WriteRelations tests removed); coverage floors hold;
arch-lint/comment-lint/lint clean.
