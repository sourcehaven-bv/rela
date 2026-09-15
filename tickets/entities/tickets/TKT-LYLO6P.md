---
id: TKT-LYLO6P
type: ticket
title: Lua and MCP opt in to a query scope by name
kind: enhancement
priority: medium
effort: m
status: backlog
---

Split out of TKT-EVR2TU (AC7). Query scopes are declared per entity type in
schema.yaml and applied to SPA surfaces; non-SPA surfaces deliberately do NOT
inherit the default (TKT-EVR2TU AC6, pinned by one test per surface). This
ticket adds the opt-IN half: a script or tool author who WANTS the filtered set
can name a scope.

## Why this is separate

AC7 is purely additive — nothing regresses without it, and the parent ticket's
"non-SPA surfaces see the unfiltered graph" property is already proven. The
implementation cost is not in the filtering (that logic exists) but in the two
seams it has to cross, both of which deserve their own review:

1. **`lua.ReadDeps`** may not import `predicate`/`predicatefns`/`scopes` under
arch-lint, so the resolver arrives as a consumer-side interface supplied at the
wiring site — 7 construction sites, including the ACL-sensitive ones. The nil
contract needs care: for a READ GATE nil means DENY (RR-X9NVHI), but a query
scope is UX rather than access control, so nil here must mean "no scoping"
without weakening the neighbouring field's rule. Getting those two nil meanings
adjacent in one struct is the part worth reviewing on its own.

2. **The MCP tool schema** gains a `query_scope` argument, which changes the
`get_metamodel`/tool-listing golden and is client-visible surface.

## Scope

### In scope

- `rela.list_entities("taak", {query_scope = "archief"})` — the options table
already rejects unknown keys, so the argument slots in beside `filter`/`limit`.
- `list_entities` over MCP taking an optional `query_scope`.
- An unknown scope name REFUSES, as everywhere else — never a silent fallback
to unfiltered. AC3's reasoning applies unchanged: a typo showing archived
records to everyone is the failure this whole feature exists to prevent.
- `query_scope: "all"` resolves and returns the unfiltered set.
- Tests per surface, mirroring the AC6 tests that pin the opposite property.

### Out of scope

- Any change to the DEFAULT behaviour of these surfaces. They stay unfiltered
unless a scope is named; that is TKT-EVR2TU AC6 and is settled.
- Other Lua read bindings (`rela.search`, `rela.get_relations`). One binding
first; widen once there is a second real case.
- Scopes for `analyze_*`, `rela validate`, tracer or export. Those answer "what
is true", and an opt-in there would invite exactly the
reports-clean-over-unseen-data mistake AC6 guards against.

## Acceptance criteria

**AC1 — Lua opts in.** A script naming a declared scope gets the filtered set;
the same script with no `query_scope` gets everything.

**AC2 — MCP opts in.** Same, over the tool argument.

**AC3 — unknown name refuses.** On both surfaces, naming an undeclared scope is
an error the caller sees, not an unfiltered result.

**AC4 — `all` withdraws.** Resolves on both surfaces even for a type declaring
no scope of that name, returning the unfiltered set.

**AC5 — the nil seam does not weaken the read gate.** A wiring site that
supplies no resolver applies no scope and still reads through `VisibleReader`.
Pinned by a test, because this is the one place the "nil DENIES" and "nil means
unscoped" rules sit in the same struct.
