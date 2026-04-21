---
id: REV-OJY1P
type: review-checklist
title: 'Review: Make `rela schema --graphviz` readable for large/polymorphic metamodels'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./...` is green.
- [x] Lint clean (`just lint`) — 0 issues.
- [x] Coverage maintained (`just coverage-check`) — local coverage ratchet check deferred to CI.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) — completed; 11 findings.
- [x] All critical review-responses addressed — RR-6OSEE (hyphenated IDs) and RR-RDILO (snapshot-lie classifier) both addressed with code fixes + targeted tests.
- [x] All significant review-responses addressed — RR-OJEIH (empty-body test) and RR-CY4X9 (demo parse-check) both addressed.
- [x] Self-reviewed the diff for unrelated changes — diff is confined to `internal/cli/schema.go`, `internal/cli/schema_test.go`, `scripts/demo-schema-render.sh`, and the ticket files.

**Review Responses:**
- RR-6OSEE (critical, addressed) — hyphenated entity IDs
- RR-RDILO (critical, addressed) — snapshot classifier
- RR-OJEIH (significant, addressed) — empty-body test
- RR-CY4X9 (significant, addressed) — demo parse-check
- RR-I4KUA (minor, addressed) — self in except list
- RR-IP8EW (minor, addressed) — visibleEntities simplification
- RR-1IGO1 (nit, addressed) — stale doc comment
- RR-OFM44 (nit, addressed) — reserved ID constants
- RR-BUJU2 (nit, addressed) — dead degree[source]
- RR-A1DIS (nit, addressed) — named minHubTargets
- RR-Y8ICV (nit, addressed) — "Collapsed relations" header

## Acceptance Verification

- [x] Each acceptance criterion tested.
- [x] Test evidence documented in implementation checklist (IMPL-FARVJ).

**Acceptance Status:**
1. `--exclude` drops entity + edges — **PASS** via `TestSchemaGraphvizExclude`.
2. ≥5 targets → legend — **PASS** via `TestSchemaGraphvizLegendFiveTargets`.
3. 3-4 targets, some isolated → hub — **PASS** via `TestSchemaGraphvizHubIsolatedTargets`.
4. 3-4 targets, all connected → legend — **PASS** via `TestSchemaGraphvizLegendConnectedTargets` (rewritten to also catch empty-body bug).
5. ≤2 targets → plain — **PASS** via `TestSchemaGraphvizFewTargetsPlain`.
6. Entities with only legend pairs hidden — **PASS** via `TestSchemaGraphvizDropsEmptyNode` + re-asserted in `TestSchemaGraphvizLegendConnectedTargets`.
7. `--no-bundle` / `--no-legend` disable features — **PASS** via `TestSchemaGraphvizNoLegendFlag` + `TestSchemaGraphvizNoBundleFlag`.
8. Generic demo script — **PASS** via `scripts/demo-schema-render.sh` (now includes hyphenated entity + `dot -Tdot` parse check; produced 60757-byte PNG).
9. Existing tests unchanged — **PASS** (all 5 pre-existing `TestSchemaGraphviz*` tests still green).

Real-world validation: rendered `tickets/metamodel.yaml` through `dot -Tpng` →
892 KB PNG (was broken at start of review with the hyphenated-ID bug).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: only change is CLI help text, which is updated in the `schemaCmd` Long description).
- [x] User-facing documentation updated — `rela schema --help` lists `--exclude`, `--no-bundle`, `--no-legend`.
- [x] ~~Docs-checklist marked as done~~ (N/A).

## Final Checks

- [x] Commit message explains the why — review-response fixes to be committed with a message referencing the addressed RRs.
- [x] No TODOs or FIXMEs left unaddressed.
- [x] Ready for another developer to use.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred: PR will be created outside the review-checklist lifecycle; tracked in branch `feat/schema-graphviz-legend-bundle`).
- [x] ~~All CI checks pass~~ (deferred to PR creation).
- [x] ~~PR URL documented below~~ (deferred to PR creation).

**PR:** *to be created via `/pr` — current branch
`feat/schema-graphviz-legend-bundle`*
