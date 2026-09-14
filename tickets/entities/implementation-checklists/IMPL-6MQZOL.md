---
id: IMPL-6MQZOL
type: implementation-checklist
title: 'Implementation: rela init generates a schema.yaml that fails to load, making new projects unusable'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: the change adds YAML keys to template string literals; no new error paths. The test propagates the detect error via `t.Fatalf`.)

`TestInitializeWithFS_NeedsNoMigration` drives the real
`projectsetup.Initialize` against a `MemFS` and then checks the result through
`projectsetup.DetectMigrationsWithFS`, the same path `rela migrate --check`
uses. It is an integration test of the init path, not an assertion about the
template constant.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The test asserts zero detections rather than matching the `short-id-default`
message, and reports the offending filename from the detection itself. Both keep
it sensitive to future migrations instead of pinning today's one. After review
it was rewritten to go through `DetectMigrationsWithFS` so it covers all three
file types (schema.yaml, data-entry.yaml, acl.yaml) rather than hardcoding
`FileTypeMetamodel` against schema.yaml.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Before, with a binary built from the parent commit, in a freshly-initialized
directory: `rela list requirement` failed with `load metamodel: ... uses
deprecated syntax`. Every command failed the same way, not just `migrate
status`.

After, in a fresh `/tmp/postfix_check`:
- `rela list requirement` → "No entities found" (loads cleanly)
- `rela migrate status` → "data schema in sync (shape 3826dbe68a57)"
- `rela validate` → "All configuration files are valid."
- `rela create requirement -P title="Test req" -s draft` → created `REQ-CGLP`,
confirming short IDs actually mint rather than the schema merely parsing.

Other template copies, via `rela migrate --check`: both `prototypes/data-entry`
and `prototypes/data-entry/catalog` now report "No migrations needed".

Test proven non-vacuous: stripping the four `id_type` lines from the template
makes it fail with `generated schema.yaml needs migration "short-id-default"`,
and pass again once restored.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

On DRY: the underlying problem is four hand-maintained schema generators
(`DefaultMetamodelYAML()`, `scripts/generate-test-data.sh`, the rela-desktop
minimal schema, and the doc examples). Unifying them is a refactor rather than
part of this fix, so each was corrected in place and the desktop one filed as
TKT-6WPC86. The regression test is what stops the init copy drifting again.

`DefaultMetamodel()`, the Go-struct twin of the YAML template, leaves `IDType`
empty, which `GetIDType()` resolves to `short` — already consistent. Verified
with a throwaway test, then deleted it.

Temporary probe tests and the scratch git worktree used to build the pre-fix
binary were removed.
