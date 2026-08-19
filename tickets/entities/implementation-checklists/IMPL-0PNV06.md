---
id: IMPL-0PNV06
type: implementation-checklist
title: 'Implementation: Aggregate-over-hidden-rows documents: elevated document renders whose output is a derived statistic'
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

### All five slices complete

**Slice 1 — read-only elevation in `internal/lua`** (15241008). `bypass_acl`
registers when EITHER handle is wired; write methods extracted to
`registerElevatedWrites` and called only when `em != nil`. AC1-AC3 ✅.
Retired `TestElevatedRead_AbsentWithoutBypassBinding` (it pinned the invariant
this ticket supersedes), rewritten as `_AbsentWithoutAnyHandle` keeping its
real concern; added `_ReaderOnlyHandle`.

**Slice 2 — `allow_acl_bypass` enum + migration** (b1c63d82).
`metamodel.ACLBypass` with `AllowsRead/AllowsWrite/Enabled`; legacy bool
REFUSED at parse time naming `rela migrate`. A test caught a real defect: a
YAML bool decodes cleanly INTO a string, so the bool branch was unreachable and
operators got a generic error — now caught by value across every YAML 1.1
spelling. `ACLBypassEnumMigration` rewrites `true` ⇒ `read+write`, drops falsy,
idempotent.

**Slice 3 — document opt-in + validation** (1f11414b). `read` only; `write`/
`read+write`/`command:` renderer are config errors; elevation REQUIRES
`permission:`. AC6 ✅. Rewrote the `Permission` godoc with both meanings.

**Slice 4 — the gate and wiring** (d53d8372). `authorizeElevatedDocument`, a
closed switch on the ACL implementation (AC7 ✅), NopACL denies (AC8 ✅),
renderer never reached on deny (AC9 ✅), elevation resolved per render with the
audit recorder travelling with the reader (AC10 ✅). `elevationRecorder`
duplicates appbuild's nil-conversion fix (typed-nil in an interface field).

**Slice 5 — docs** (92b12717). `lua-scripting.md`, `data-entry.md`,
`acl-security.md` (the RR-LWD8N3 trust boundary stated plainly), nested
`CLAUDE.md`.

**arch-lint fix** (ef82aa3f). The enum threaded a metamodel import into
`autocascade`, which may not have one. `ScriptAction.AllowACLBypass` is a plain
string; `script` converts once.

**Manual verification.** `TestElevatedDeps_GrantsBypassBinding` is the wiring
proof the other tests could not give: it builds a REAL `lua.Runtime` from the
deps `elevatedDeps` produces and asserts from inside Lua that an elevated
render has `bypass_acl` with three read methods and NO write methods, and that
an unelevated one has no binding at all. The handler tests use a fake renderer,
so they prove the gate; this proves the capability actually arrives.

**Deviation from plan.** AC11 (audit row survives a raising closure) is covered
by the existing `defer` in `luaBypassACL`, exercised by the `internal/lua`
suite; no document-specific test was added, since the recorder path is
identical and the defer is not surface-specific.

**Full CI gate green:** `go test ./...`, `golangci-lint run ./...` (0 issues),
`just arch-lint` (OK), `just plimsoll`, `just coverage-check` (77.7%, both
thresholds satisfied), `markdownlint` on every changed doc.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
