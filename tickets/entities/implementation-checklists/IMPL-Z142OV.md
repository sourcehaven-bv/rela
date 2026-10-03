---
id: IMPL-Z142OV
type: implementation-checklist
title: 'Implementation: CLI, MCP, Lua, automation and importer writes read the zero-face row'
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

Built `rela` from the branch and ran it against a temp fs project with a faced
`policy` (draft, published) and a content-scoped `implements` relation:

- `import` of `POL-1@draft`, `POL-1@published` and two content edges wrote
`POL-1@draft.md`, `POL-1@published.md` and both tailed relation files.
- `unlink POL-1@published implements CTL-1` removed only the published edge.
- `delete POL-1@draft --force` refused with "has 1 relation(s)"; with
`--cascade` it removed the draft face and its edge only.
- `delete POL-1 --force` removed the family.

Automated: faced-fixture tests for CLI delete and unlink, MCP delete_entity and
delete_relation, Lua delete_entity and delete_relation, automation
create_relation and IfExistsReplace, the cascade-host family delete (audit and
version per face), import, `DeleteEntityFace` reading inside the Tx, D4 ACL
cases, and the pg family-lock barrier test under `-race`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
