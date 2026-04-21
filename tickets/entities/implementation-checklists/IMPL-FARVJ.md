---
id: IMPL-FARVJ
type: implementation-checklist
title: 'Implementation: Make `rela schema --graphviz` readable for large/polymorphic metamodels'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units) — `scripts/demo-schema-render.sh` runs the full pipeline through graphviz
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place — there are no new error paths; no file I/O, no external calls

## Test Quality

- [x] Using fixture builders or factories for test data — new `schemaGraphvizFixture` helper.
- [x] No hardcoded values in assertions when object is in scope — assertions use identifiers directly from fixtures.
- [x] Only specifying values that matter for the test — the fixture helper accepts minimal `map[string]EntityDef` / `map[string]RelationDef`, no boilerplate.
- [x] Interpolated values constructed from objects, not hardcoded — N/A here (no interpolation scenarios).
- [x] Property comparisons use original object, not hardcoded strings — N/A (tests assert structural DOT output, which is the primary artifact).

## Manual Verification

- [x] Feature manually tested end-to-end — ran `scripts/demo-schema-render.sh`, rendered `/tmp/rela-schema-demo.png` through graphviz.
- [x] Each acceptance criterion verified with a test scenario from planning.
- [x] Edge cases manually verified — zero relations, single entity, mixed isolation, `--exclude nonexistent`, HTML-escape on labels.

**Verification Evidence:**

Local runs:
```
go test ./internal/cli/ -run TestSchemaGraphviz -v   # all pass
go test ./internal/cli/ -run TestFormatTargets -v    # all pass
just lint                                             # 0 issues
scripts/demo-schema-render.sh                         # all assertions ✓, PNG 43175 bytes
```

Acceptance criteria map to tests:

| AC | Test |
|----|------|
| 1 | `TestSchemaGraphvizExclude` ✓ |
| 2 | `TestSchemaGraphvizLegendFiveTargets` ✓ |
| 3 | `TestSchemaGraphvizHubIsolatedTargets` ✓ |
| 4 | `TestSchemaGraphvizLegendConnectedTargets` ✓ |
| 5 | `TestSchemaGraphvizFewTargetsPlain` ✓ |
| 6 | `TestSchemaGraphvizDropsEmptyNode` ✓ |
| 7 | `TestSchemaGraphvizNoLegendFlag` + `TestSchemaGraphvizNoBundleFlag` ✓ |
| 8 | `scripts/demo-schema-render.sh` ✓ — assertions + valid PNG |
| 9 | Existing `TestSchemaGraphviz*` suite ✓ — 5 tests unchanged |
| extras | `TestFormatTargets` (table) + `TestSchemaGraphvizEscapesHTML` ✓ |

## Quality

- [x] Code follows project patterns — reuses existing `getSortedEntityNames`, `getSortedRelationNames`, `darkenColor`, `defaultEntityColors`, `defaultEdgeColors`, `buildConstraintLabel`.
- [x] No security issues introduced — HTML-escape for user-provided labels in the legend's HTML-like TABLE; no file / subprocess / network operations.
- [x] No silent failures — classification is deterministic; the renderer either emits edges, hub, or legend based on flag-and-structure rules.
- [x] No debug code left behind.
