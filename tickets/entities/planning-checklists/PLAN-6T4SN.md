---
id: PLAN-6T4SN
type: planning-checklist
title: 'Planning: Make `rela schema --graphviz` readable for large/polymorphic metamodels'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

*In scope:*
- `--exclude <type>` (repeatable) — drop entity type and associated edges.
- `--no-bundle` / `--no-legend` flags to disable the two rendering features.
- Classification per `(source, relation)` pair:
  - ≤ 2 targets → plain edges (unchanged)
  - 3 or 4 targets, **at least one target otherwise isolated** → hub-bundle
  - 3 or 4 targets, **all targets otherwise connected** → legend
  - ≥ 5 targets → legend
- "Otherwise connected" is computed on the post-exclude graph: a target is connected if it has ≥ 1 other edge besides the one being classified.
- Legend: single `__legend` plaintext node, HTML-like TABLE with per-entry 2-row block (header `LTR` + target list `LBR`), left-aligned.
- Smart target-list formatting: full list / "any entity except X, Y" / "any entity".
- Empty entity nodes (fully collapsed with no other edges) are dropped.
- Generic demo script exercising all four classification buckets, validated end-to-end through graphviz.

*Out of scope:*
- `-o` / `-f` rendering flags on `schema --graphviz`.
- Layout engine changes.
- `rela graph` changes.

**Acceptance Criteria:** see ticket body — 9 specific criteria, one per test.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions explored:**
- Graphviz `concentrate=true` — didn't improve readability when tested.
- Force-directed layouts (`fdp`/`sfdp`/`neato`) — sacrifice hierarchical flow.
- Hub synthesis with `minlen` — over-constrains layout; default hub placement reads better.
- HTML-like TABLE labels — stable across `dot` versions, chosen for the legend.

**Codebase:**
- Flag/dispatch pattern: `internal/cli/schema.go:54-55`, registration: `schema.go:693-694`.
- Entity/relation iteration: `getSortedEntityNames`, `getSortedRelationNames`.
- Existing generator: `runSchemaGraphviz` in `schema.go:507` (~60 lines).
- Test fixture pattern: `internal/cli/schema_test.go:641+`.

**Prior art:** the PR-522 DOT sanitization fix — informs safe identifiers for
the `__legend` and `__hub_N` nodes.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Extend `runSchemaGraphviz` with four passes before DOT emission:

1. **Apply excludes.** Filter `entityNames` to drop excluded types. For each relation, strip excluded types from `From` / `To`; drop the relation entirely if either side becomes empty.

2. **Build the adjacency index.** For each entity type, precompute an *all-edge count* across every `(source, relation)` pair after excludes — counting edges that would be emitted if every pair were plain. This is the "otherwise connected" snapshot and must be built before classification to avoid feedback.

3. **Classify each (source, relation) pair** into one of three outputs:
   - `plain` — ≤ 2 targets, OR 3-4 targets with at least one otherwise-isolated → wait, opposite: classification by target count:
     - **≤ 2 targets:** plain.
     - **3 or 4 targets:** check each target's other-edge count in the adjacency index (subtracting the edge being classified). If every target has ≥ 1 other edge → `legend`. Else `hub`.
     - **≥ 5 targets:** `legend`.
   - `hub` — emit `__hub_N [shape=point, width=0.05, height=0.05, label=""]`, `source → __hub_N [label="rel", arrowhead=none]`, and one `__hub_N → target` per target (no label on the inner edges).
   - `legend` — accumulate `(source-label, relation-label, targets)` into a slice; emit no edges.

Two-pass ordering prevents chicken-and-egg: the adjacency count is computed
assuming *all* pairs plain, so classifying A→B as legend doesn't retroactively
make A→B look different.

4. **Emit.** Entities first (skipping those with zero final attachments). Plain edges and hubs next. Legend node last.

**Files to modify:**
- `internal/cli/schema.go` — flags, extend `runSchemaGraphviz`. New helpers: `filterExcluded`, `countOtherEdges`, `classifyPair`, `renderHub`, `renderLegend`, `formatTargets`.
- `internal/cli/schema_test.go` — table tests for each classification bucket, `--exclude`, `--no-*` flags, empty-node drop, HTML escape.
- `scripts/demo-schema-render.sh` — synthetic metamodel covering all four buckets; runs through `dot -Tpng`; fails on empty PNG.

**Alternatives rejected:**
- Ratio-based universal threshold: user chose the bucket-based rule (≤2 / 3-4+isolated → hub / 3-4+connected → legend / ≥5 → legend) over a ratio because it's structural, not heuristic.
- Tiered legend sections: start simple, add later if needed.
- Post-processing DOT in a separate binary: less convenient.

**Dependencies:** no new Go packages. `html.EscapeString` for label escaping in
TABLE content.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `--exclude <type>` — string equality against `meta.Entities` keys; no DOT interpolation beyond already-sanitized identifiers.
- Entity / relation labels embedded in the legend TABLE — HTML-escape via `html.EscapeString`.

**Security-Sensitive Operations:** none. No file writes, no subprocess, no
network.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
|----|------|
| 1 | `TestSchemaGraphvizExclude` — 3-type metamodel, `--exclude` one, assert absence. |
| 2 | `TestSchemaGraphvizLegendFiveTargets` — 5 targets; assert `__legend` row + zero edges for that relation. |
| 3 | `TestSchemaGraphvizHubIsolatedTargets` — 3 targets, each with no other edges; assert `__hub_` node + correct edges. |
| 4 | `TestSchemaGraphvizLegendConnectedTargets` — 4 targets each with other edges; assert `__legend` row + zero edges for that relation. |
| 5 | `TestSchemaGraphvizFewTargetsPlain` — 2 targets; plain edges. |
| 6 | `TestSchemaGraphvizDropsEmptyNode` — entity whose only outgoing relation goes to legend and has no incoming; assert node absent. |
| 7 | `TestSchemaGraphvizNoBundle` / `NoLegend` — flags disable; output reverts. |
| 8 | `scripts/demo-schema-render.sh` — end-to-end graphviz, exit 0, non-empty PNG. |
| 9 | Existing `TestSchemaGraphviz*` suite — unchanged. |
| extra | `TestFormatTargets` — table test over (targets, total): exactly-all / total-1 / total-2 / else. |
| extra | `TestSchemaGraphvizEscapesHTML` — entity label contains `<>&"`; escaped in legend. |

**Edge Cases:**
- Zero relations → no hubs, no legend.
- Relation with multiple `from` types — each `(from, relation)` classified independently.
- `--exclude` of a non-existent type — silently ignored.
- Target count exactly at boundary (= 2, = 5) — `> 2`, `>= 5` tested.
- Mixed isolation in the same target set (e.g. 3 targets: 2 connected, 1 isolated) — falls into the "hub" branch because *at least one* is isolated.

**Negative Tests:** none required beyond argument parsing (no numeric input to
validate).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|------|------------|
| Classification ordering (A reclassifies B reclassifies A) | Two-pass: build adjacency index assuming all-plain; classify from that frozen snapshot. |
| Graphviz versions render HTML-like labels differently | End-to-end demo through `dot` in CI (graphviz already installed from PR-522). |
| Hub positioning suboptimal | Accept graphviz defaults; revisit if users hit problems. |
| HTML-escape bugs in labels | `html.EscapeString`; explicit test. |

**Effort:** s (~150 LOC Go incl. helpers, ~200 LOC tests, ~60 LOC shell demo;
2-3h).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] CLI help text updated in `schema.go`'s Long description.
- [x] ~~User guide / reference~~ (N/A).
- [x] ~~CLAUDE.md~~ (N/A).
- [x] ~~README.md~~ (N/A).
- [x] ~~API docs~~ (N/A: CLI-only).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design validated iteratively in chat via working prototype; user signed off on bucket-based rule).
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no formal review run).

**Design Review Findings:** none — design validated through working prototype +
user-directed rule refinement in chat.
