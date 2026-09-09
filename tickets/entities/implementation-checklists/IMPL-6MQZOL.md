---
id: IMPL-6MQZOL
type: implementation-checklist
title: 'Implementation: rela init writes a schema.yaml that rela migrate immediately flags as deprecated'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: the change is four lines of YAML in a string literal; no new error paths. The test propagates `Detect`'s error via `t.Fatalf`.)

`TestInitializeWithFS_SchemaNeedsNoMigration` drives the real
`projectsetup.Initialize` against a `MemFS` and asserts on the file it actually
writes, so it is an integration test of the init path rather than an assertion
about the template constant.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The test reads `result.SchemaPath` off the returned `InitResult` rather than
re-deriving the path, and asserts zero detections rather than matching the
`short-id-default` message. Both keep it sensitive to future migrations instead
of pinning today's one.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Before: `rela init` then `rela migrate status` in an empty dir reported "uses
deprecated syntax: Add explicit id_type: sequential...".

After, in a fresh `/tmp/initest2`:
- `rela migrate status` → "data schema in sync (shape 3826dbe68a52)"
- `rela validate` → "All configuration files are valid."
- `rela create requirement -P title="Test req" -s draft` → created `REQ-CGLP`,
confirming short IDs actually mint rather than the schema merely parsing.

Test proven non-vacuous: stashing the template fix makes it fail with `generated
schema needs migration "short-id-default"`, then pass again once restored.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Checked that the YAML template and its Go twin `DefaultMetamodel()` agree: the
struct leaves `IDType` empty, which `GetIDType()` resolves to `short`. Verified
with a throwaway test, then deleted it — the duplication between the two
representations is pre-existing and out of scope here. Temporary probe tests
used during investigation were removed; `git status` shows only the two intended
code files.
