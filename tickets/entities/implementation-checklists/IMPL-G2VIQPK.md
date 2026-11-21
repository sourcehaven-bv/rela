---
id: IMPL-G2VIQPK
type: implementation-checklist
title: 'Implementation: Portal polish: label sources; tab title; currency digits; git status request'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (schema_labels_test.go, format, title, git store, widget registry)
- [x] Integration tests written (TestSchemaCapabilities_Git over the handler; golden wire files updated)
- [x] Happy path implemented
- [x] Edge cases from planning handled (own label wins, incoming inverse label, JPY digits, unmount during hold)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: small inline schema)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
On a copy of the harbourmaster project served by rela-server from the
branch: the account view's status field without `label:` reads "Status",
the schema label the form shows; the invoice table shows €49.00 and €59.29;
/_schema reports `git: false` and the SPA sends no /_git/status request.

## Quality

- [x] Code follows project patterns (copy-on-serve like resolveListDirections)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (labels are config, not data)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
