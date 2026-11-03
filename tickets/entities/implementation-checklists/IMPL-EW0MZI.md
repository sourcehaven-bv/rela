---
id: IMPL-EW0MZI
type: implementation-checklist
title: 'Implementation: Split metamodel.Metamodel part 2: relation and property-type queries'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestMigrationSchema, TestLoadMigrationMetamodel_MissingSchemaIsNilInterface, TestResolveWidgetFromType in metamodel)
- [x] ~~Integration tests written~~ (N/A: no new behaviour beyond the nil fix; existing migration, dataentry, cli and entitymanager suites exercise every moved call)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: small literal metamodels, as the package tests already do)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: no interpolated assertions)
- [x] ~~Property comparisons use original object~~ (N/A: none)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
1. `go test ./...`: all pass except TestChromeStyle_TargetsShippedClasses (needs a built SPA; fails the same on develop). Touched packages also pass with the sqlite tag. TestLoadMigrationMetamodel_MissingSchemaIsNilInterface fails with the nil guard removed.
2. `just plimsoll` passes with both directives removed (Metamodel 11 exported, EntityDef 17).
3. `rela schema overview|entities|relations|types|entity ×3|relation ×3 -o json` on the tickets project: byte-identical before and after (diff -r).
4. arch-lint OK; golangci-lint 0 issues on all four tag sets; tests compile under default, memorybackend, postgres, sqlite.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (none; method bodies moved unchanged)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
