---
id: PLAN-BW4BTQ
type: planning-checklist
title: 'Planning: Read-only graph access in Lua migration steps: rela.get_entity and rela.get_relations'
started: "2026-10-13"
completed: "2026-10-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:
- `rela.get_entity(addr)` and `rela.get_relations(opts)` in the `lua:`
migration sandbox, reading the raw store.
- Consistency: every read in a step sees the store as it was before the step.
- Docs in the data-migration guide (GUIDE-data-migration, rendered to
docs/data-migration.md), with a classify-by-relation example.

OUT:
- Any write function. The script stays a pure transform.
- `list_entities`, `search`, `trace_*` and `find_path`. They can be added on
the same reader when a migration needs them.
- Making a dry-run show the effect of earlier steps on later ones. A dry-run
writes nothing, so every step of a dry-run reads the pre-run store. That is how
dry-run already behaves for every step kind; the follow-up ticket on the dry-run
report records it.

**Acceptance Criteria:**

1. A migration can classify an entity by an outgoing relation to an entity of
a given type. Test: tasks with an `assigned-to` edge to a `person` get `kind =
"assigned"`, the rest `kind = "general"`.
2. Reads within a step see the pre-step state, whatever the iteration order.
Test: one step patches every task; each task records what `get_entity` returns
for the task processed before AND after it. Every recorded value is the pre-step
value.
3. A later step sees the results of an earlier step (apply). Test: step 1 sets
a property, step 2 copies it from another entity via `get_entity`.
4. Re-running is a no-op. Test: run twice; the second run affects 0 records.
5. Reads ignore validation and ACL. Test: an entity of a type the new schema
does not know, and one that fails validation, are both returned by `get_entity`.
6. `entity: "*"` works with reads. Test: a `*` step reads relations of every
entity.
7. Works on memstore, fsstore, sqlitestore and pgstore (pg when
RELA_TEST_DATABASE_URL is set; CI's Postgres job runs it).
8. No write function is reachable. Test: `rela` holds exactly
`get_entity` and `get_relations`; `rela.update_entity` is nil.
9. Faces: `get_entity("ID@face")` reads that row; a bare id of a faced entity
with several stored faces raises an error naming them; a bare id with one row
returns it; a missing id returns nil.
10. `get_relations` rejects a non-string option and a non-table argument.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small, bounded change)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `internal/lua/runtime.go` `luaGetRelations` / `relationQuery` /
`relationToTable` / `EntityToTable`: the normal API. The migration binding
mirrors their argument checks and table shape. It does not import
`internal/lua`: datamigration's arch rule allows only gopher-lua, and
`entityToTable` in luastep.go already mirrors locally for the same reason.
- `store.Family` reads every face row of an id (raw).
- `entity.ParseAddress` parses `ID` / `ID@face` without a bare `entity.Ref`
literal (archguard bare-ref rule).
- `Exec.forEachEntity` (run.go) is collect-then-write: it calls the step
function for every entity and only then writes the changed ones in Tx batches.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- `luaStep.Run` installs a `rela` table with two closures over a narrow,
consumer-side `graphReader` interface (`GetEntity`, `ListEntities`,
`ListRelations`) satisfied by `x.Store`. The interface has no write method, so
the binding cannot write even by mistake.
- Snapshot consistency comes from the runner's existing collect-then-write
order. `forEachEntity` finishes every `migrate()` call before it writes
anything, so every read in the step sees the pre-step store. Stores return
copies (memstore clones, fsstore/sqlite/pg decode fresh rows), so the in-place
patch of the handed entity cannot leak into a later read. Writes of step N land
before step N+1 starts, so later steps see earlier results. The ordering is now
load-bearing, so `forEachEntity`'s doc comment states it and a test pins it.
- `get_entity(addr)`: `entity.ParseAddress`. Named face: `GetEntity` at that
ref, `ErrNotFound` maps to nil. Bare id: `store.Family`; 0 rows nil, 1 row that
row, more than 1 raises naming the faces. There is no world in a migration to
rank faces, and picking one silently would make a classification depend on an
arbitrary face.
- Result tables: entity `{id, type, face, content, properties}`; relation
`{from, type, to, from_face, content, properties}`. `properties` is always
present.
- `get_relations(opts)`: same keys and non-string rejection as the normal API.
A non-table, non-nil argument raises (the normal API silently returns every
relation for `get_relations(id)`, a documented trap; a migration has no reason
to keep it). A store error raises rather than truncating.

**Alternatives considered:**

- In-memory overlay or full graph copy per step. Rejected: duplicates store
semantics and holds the whole graph (pg tenants hold GBs of content) in memory,
while collect-then-write already gives the guarantee.
- A pg `REPEATABLE READ` transaction for the read phase. Rejected: needs a new
per-backend seam; fs has no equivalent; and the migration lock already excludes
other migration writers.
- Reuse `internal/lua` runtime with `AllowAllReader`. Rejected: drags the
whole binding surface (and write functions) into the sandbox, against the
pure-transform rule.

**Files to modify:**

- internal/datamigration/luastep.go (bindings)
- internal/datamigration/run.go (doc comment on the ordering guarantee)
- internal/datamigration/luastep_reads_test.go (new)
- docs-project/entities/guides/GUIDE-data-migration.md, docs/data-migration.md
- .go-arch-lint.yml (comment only), .github/workflows/ci.yml (pg leg)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

The script is operator-authored and reviewed in the migration file, the same
trust as today. Arguments are type-checked (strings only); addresses go through
`entity.ParseAddress`.

**Security-Sensitive Operations:**

Reads bypass ACL. That is the runner's existing trust model (operator shell, or
a `config:edit` holder on a Configure save, which is admin-equivalent). The
sandbox still has no io/os/debug and no write handle.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one test per acceptance criterion above, through
`Runner.Run` (the same entry the CLI uses), so the tests cover parsing, step
execution and write-back together. AC 7 runs the classification and snapshot
tests against each backend.

**Edge Cases:** missing id (nil), malformed address (error), empty id (error),
entity with several faces (error), relation with properties and content,
`get_relations()` with no argument (all relations).

**Negative Tests:** non-string option, non-table argument, ambiguous bare id.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Someone later changes `forEachEntity` to write as it goes. Mitigation: the
snapshot test fails, and the doc comment names the dependency.
- Crash mid-step: the re-run sees batches already written. A script that
decides from a property the same step rewrites could then classify differently.
Mitigation: documented; base decisions on data the step does not change
(relations, other types). Atlas's case reads relations only.
- Cost: one store query per call per entity. Acceptable for an operator-run
offline step; documented.
- Concurrent non-migration writers (a running server) can change data during
a step. The migration lock does not exclude them today; the iteration itself has
the same exposure. Documented, not changed.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- docs-project GUIDE-data-migration → docs/data-migration.md: "The Lua escape
hatch" gains a "Reading the graph" subsection with the example and the
consistency rules.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Self-review against the design-review prompts; no
critical or significant findings. The dry-run limitation and crash re-run caveat
were added to scope and risks as a result.
