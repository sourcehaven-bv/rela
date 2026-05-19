---
id: IMPL-WA2FN
type: implementation-checklist
title: 'Implementation: ACL v0: declarative write-side enforcement with delegate-X tamper resistance'
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

- **`internal/acl/` unit tests** (`acl_test.go`, `nop_test.go`, `readonly_test.go`): 100% line coverage. Verifies AC1.1 (errors.Is), AC1.2 (NopACL allow-all across 8 request shapes), AC1.3 (ReadOnlyACL deny-all + correct Decision fields across 7 shapes).
- **`internal/entitymanager/acl_test.go`** — `TestManager_ACLDenies_AllWritePathsBlocked` table-driven across all 7 write entry points (Create/Update/Delete × Entity/Relation + Rename). Each subtest asserts: returned error satisfies `errors.Is(_, ErrForbidden)`, `errors.As` extracts the Decision, store call counters did not change, exactly one new `denied-write` audit row with the correct `Subject.Kind`. Verifies AC1.4 + AC1.5 + AC1.6.
- **`internal/dataentry/acl_test.go`** — `TestHandler_ACLDeny_Returns403Structured` boots an in-memory App with `ReadOnlyACL` via `appbuild.WithTestACL`, POSTs a create-entity request via `httptest`, asserts status 403, `Content-Type: application/json`, and the structured body has `error="forbidden"`, `rule_kind="read-only"`, `rule_id="read-only-acl"`, non-empty `reason`. Verifies AC1.7.
- **`internal/appbuild/acl_test.go`** — verifies `appbuild.WithACL(acl.ReadOnlyACL{})` produces a Services whose Manager denies; default uses `NopACL`. Verifies AC1.8 wiring at the seam (the `--read-only` flag in main is trivial after this).
- **Live smoke test against `tickets/`** with 1249 entities, 1429 relations:
  - `rela-server --read-only --port 8765` boots; startup log: `WARN rela-server is read-only; every write request will be refused`.
  - `curl http://127.0.0.1:8765/api/v1/tickets?limit=1` → 200 (read unaffected).
  - `curl -X POST http://127.0.0.1:8765/api/v1/tickets -d '{...}'` → 403 with body `{"error":"forbidden","reason":"this rela instance is configured read-only","rule_id":"read-only-acl","rule_kind":"read-only"}`.
- **All existing tests** in `internal/entitymanager`, `internal/appbuild`, `internal/attachment`, `internal/mcp`, `internal/cli`, `internal/dataentry` rebaselined to pass `acl.NopACL{}` and remain green — no behavior change for projects without `acl.yaml`.
- **`just ci` exits 0** end-to-end (test + lint + arch-lint + coverage + build + docs + frontend).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
