---
id: IMPL-3FJKAN
type: implementation-checklist
title: 'Implementation: World chrome speaks the operator''s words or stays silent: messages, on_absent redirect, copy on_success'
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

**Verification Evidence:** metamodel: field-coverage tests extended for
`WorldDef.Messages/OnAbsent`, `FaceDef.Messages`, `CopyDef.OnSuccess`;
`TestWorlds_OnAbsentRedirectValidated` and `TestValidateCopies_Landing` pin the
load errors. dataentry: `schemamessages_test.go` pins the wire projection
(omitted when undeclared). SPA: `worldText` unit tests; WorldBadge, EntityList,
KanbanView, RelationPicker and EntityDetail world suites rewritten to "silence
unless declared", plus `on_absent.redirect` and every `landing` mode. Chrome
walk on the atlas verify project with Dutch messages declared: POLICY-001 shows
"Dit is de vastgestelde versie. Bewerken doe je in het concept." and nothing
else; the Beleid list shows the projection note and a "Concept" badge on the
stand-in row; the Taken list shows no banner; the form refusal carries the same
Dutch sentence and only "Back to entity"; Vaststellen toasts "Thuiswerkbeleid is
vastgesteld." and lands on the adopted face. The atlas verify manual passes with
the extended fixture (19 claims). Go suite, 2361 SPA tests, golangci-lint,
comment-lint, arch-lint, plimsoll all green.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
