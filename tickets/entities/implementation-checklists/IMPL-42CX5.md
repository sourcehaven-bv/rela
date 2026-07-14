---
id: IMPL-42CX5
type: implementation-checklist
title: 'Implementation: Relation-based validation gates are silently dropped; port workflow gates to Lua + enforce done-before-PR'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no Go code changed; the change is
a Lua validator + metamodel rules + CI/command config)
- [x] Integration tests written (test full flow, not just units) — `rela validate`
against the live tickets project IS the integration test; verified gates fire
and pass appropriately
- [x] Happy path implemented (14 gates ported to require-relation-count.lua)
- [x] Edge cases from planning handled (min with/without where; max:0; quoted YAML)
- [x] Error handling in place — the Lua validator fails loudly on malformed args

## Test Quality

- [x] ~~fixture builders / factories~~ (N/A: no unit-test code)
- [x] ~~hardcoded values in assertions~~ (N/A)
- [x] ~~only specifying values that matter~~ (N/A)
- [x] ~~interpolated values from objects~~ (N/A)
- [x] ~~property comparisons use original object~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- Before backfill, `rela validate` flagged the four gates:
done-ticket-needs-review-done 31, done-bug-needs-review-done 27,
done-enhancement-needs-docs-done 83, done-docs-ticket-needs-docs-done 7.
- After prune (114 pre-July) + backfill (15 July): `rela validate` down to only
TKT-IFHO2L's own in-flight gate trips + 3 pre-existing unrelated warnings.
- Backfilled ticket (e.g. TKT-4MKKKA with a `has-review -> done` checklist) now
passes done-ticket-needs-review-done.
- max:0 response gates stay at 0 violations.
- CI awk status parse verified on quoted and unquoted frontmatter.
- Workflow YAML validated with a Python yaml.safe_load.

## Quality

- [x] Code follows project patterns (lua_file rule mirrors validate-justification.lua)
- [x] Checked for DRY opportunities — one parameterized validator covers all 14
gates (min/max + where-filters) instead of 14 bespoke scripts
- [x] No security issues introduced (CI head_ref via env var, literal prefix
match only; Lua args are trusted metamodel config, still guarded)
- [x] No silent failures (validator returns explicit violation messages)
- [x] No debug code left behind (probe rule reverted; scratch files outside repo)
