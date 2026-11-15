---
id: IMPL-PSCQ9IY
type: implementation-checklist
title: 'Implementation: Declarative mapped import: rela import --mapping (JSON/XML/YAML/CSV, diff, Lua hook)'
started: "2026-11-14"
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

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
Built `rela` and ran it on a scratch project with the XML OSCAL catalog,
the XML mapping and the oscal.lua hook. `rela validate` reported the imports
as valid. `--dry-run` planned 6 creates, 4 relations and 1 record skipped by
the hook, and wrote nothing. The real run created 6 entities and 4 relations.
A second run planned nothing: 6 unchanged. Unit tests cover the same flow for
JSON and XML, re-import updates, clear_missing, stale and pruned relations,
missing entities, ACL denial, hidden rows, caps, hook errors, YAML and CSV.
`TestImportMapped_CLI` covers dry run, apply and re-run through the CLI.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
