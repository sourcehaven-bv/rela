---
id: IMPL-ZWMZU
type: implementation-checklist
title: 'Implementation: Add native relation-cardinality support to validation rules (relations: block on ValidationRule)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (relation_constraint_test.go:
min/max/where/bounds/no-reader/unevaluable-target/malformed-where)
- [x] Integration tests written — loader parse tests, plus a whole-corpus
parity diff of `rela validate` output against the Lua baseline binary
- [x] Happy path implemented (Relations field + evaluation + strict loader)
- [x] Edge cases from planning handled (missing reader, dangling target, empty where)
- [x] Error handling in place (a check that cannot run is REPORTED, not counted
as satisfied; unevaluable targets fail closed under `max:`; loader rejects
unknown keys, undeclared relation types and unsatisfiable bounds)

## Test Quality

- [x] Using table-driven subtests with t.Run
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- `rela validate` output identical to the Lua baseline: built the pre-migration
binary with the Lua validator restored and diffed both against the same corpus
— empty. Re-verified after the review fixes; parity still holds.
- Deliberate `relationz` typo → load fails: `validation "broken-rule": unknown
key "relationz" (valid keys: ...)`.
- Detail carries the specifics: `requires at least 1 'has-review' (status=done)
relation(s), has 0` while Description is the human rule text (confirmed in
`-o json` output).
- Scanned every schema.yaml/metamodel.yaml in the repo → no false rejections
from the strict loader (including the added relation-type and bounds checks).
- go test ./... green; coverage thresholds satisfied (validation 88.5%).

Rebase note: the branch was rebased over 360 upstream commits. `ReadDeps.Store`
had been replaced by the ACL-gated `VisibleReader`, so the relation reads were
rewired to it (same gating the Lua path already used), and ctx is now threaded
through the predicate helpers instead of `context.Background()`.

## Quality

- [x] Code follows project patterns (consumer-side ReadDeps helper; whitelist+parity-test mirror)
- [x] Checked for DRY opportunities — reused filter.MatchAll; one OutgoingRelations helper
- [x] No security issues introduced (allowlist loader; read-only traversal)
- [x] No silent failures (unknown keys, undeclared relation types and
unsatisfiable bounds are load errors; an unrunnable check reports instead of
passing; violations carry Detail)
- [x] No debug code left behind
- [x] arch-lint passes (validation does not import internal/store; reads via lua.ReadDeps helper)
