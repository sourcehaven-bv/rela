---
id: IMPL-TYHMWJ
type: implementation-checklist
title: 'Implementation: Version tags'
started: "2026-10-07"
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
Manual run on a sqlite build of the CLI in a scratch project:
- Tag right after create captures v1 before the sweep ran; `rela history` lists it.
- Update, then a namespaced tag `sync/basecamp` on v2; tags show per version.
- A bad name (`Bad Name`) is refused with the grammar in the message.
- `--delete` removes a tag; rename keeps the tag on the renamed lineage.
- Purge of the tagged row is refused without `--force-tags`; with it the tag is dropped and a tombstone written.
- Tagging the current state after it was purged is refused.
Automated: storetest RunVersionTagTests on pg and sqlite (capture-now, move, delete, rename, recreate, rename onto deleted id, concurrent delete, CAS, in-Tx refusal, purge), entitymanager authorization and audit, visibility hidden-equals-missing, Lua appbuild integration on sqlite, CLI tests, archguard Tx-context guard, sqlite v12 to v13 migration.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
