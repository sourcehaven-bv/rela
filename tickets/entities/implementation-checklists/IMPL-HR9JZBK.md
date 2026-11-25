---
id: IMPL-HR9JZBK
type: implementation-checklist
title: 'Implementation: Apply pending config migrations in memory on load and warn instead of refusing to boot'
started: "2026-11-25"
completed: "2026-11-25"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: inline YAML fixtures are the pattern in migration and metamodel tests)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- Scratch project with `id_type: auto` and `source_max:` in schema.yaml:
`rela list ticket` loads and logs one WARN naming `[cardinality-rename
short-id-default]`; `rela validate` still fails with the deprecated-syntax
message; schema.yaml is byte-identical afterwards.
- `TestFSLoader_MigratesDeprecatedSyntaxInMemory` and
`TestReloadConfig_MigratesDeprecatedSyntax` fail when the lenient path is made
strict again (checked by breaking the fix).
- `TestFSLoader_RefusesAutomations` still passes: cross-file migration refuses.
- `TestMigrateBytes_Refuses` covers an Apply error and a migration that does
not converge; `TestMigrateBytes_ReturnsInputUnchanged` covers invalid YAML and
advisory-only input.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
