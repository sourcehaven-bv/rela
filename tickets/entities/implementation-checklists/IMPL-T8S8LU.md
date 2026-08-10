---
id: IMPL-T8S8LU
type: implementation-checklist
title: 'Implementation: Unified targeted-write primitive: entitymanager.PatchEntity replaces four hand-rolled property merges'
status: done
---

<!-- @managed: claude-workflow v1 -->

Landed as 5 commits on `tkt-80ewgm-patch-entity`, sequenced so the risky
refactor is isolated and reviewable on its own:

| Commit | What |
|---|---|
| `29964269` | extract `updateCore` (behaviour-preserving, standalone) |
| `08c15888` | `entity.Patch` + `Manager.PatchEntity` + `FieldWriteGate` |
| `a3ef1817` | lua migration; **delete `ReadDeps.WritePrepStore`** |
| `8619cf72` | mcp + cli migrations |
| `c575a947` | docs: root CLAUDE.md rule, entitymanager CLAUDE.md, cli-reference |

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

New tests in `internal/entitymanager/patch_test.go` (13 funcs / 20 cases) plus
`internal/cli/update_test.go` (flag→patch translation). Integration coverage
runs through real `Deps` with automation + cascade + audit wired, not stubs.

**Two deviations from plan, both deliberate:**

1. **`entity.EntityPatch` → `entity.Patch`.** revive flagged the stutter
(`entity.EntityPatch`). Renamed before it reached any consumer.
2. **The field gate is wired as `AllowAllFieldGate` everywhere, not
policy-backed.** The real resolver lives behind
`dataentry.newFieldVerdictResolver`, and `appbuild` may not import
`internal/affordances` under arch-lint (`.go-arch-lint.yml:410-434`). Wiring it
properly means moving resolver construction out of `dataentry` — more than this
ticket scoped. **This preserves today's exact behaviour** (no path outside
dataentry gates fields today), and the seam is now in place, so it becomes a
wiring change rather than a rewrite. Flagged as a follow-up, not silently
dropped.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`newPatchManager(t, gate)` / `seedTask` / `mustGet` are the fixture helpers;
`recordingGate` records what it was asked so tests assert the gate was
*consulted*, not merely that a write succeeded.

**Mutation-tested the audit assertion** rather than trusting it: disabling
`recordEntityAudit` makes `TestPatchEntity_RunsFullPipeline` fail with "no audit
record emitted"; restoring it passes. The assertion is real, not vacuous. This
mattered because `internal/entitymanager/CLAUDE.md` warns audit is inherited by
method *name* — `PatchEntity` is not in that set, so it had to be verified
rather than assumed.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built `cmd/rela` and drove a real fs-backed project (`/tmp/clitest`), not a test
harness:

| # | Command | Result |
|---|---|---|
| 1 | `update TASK-1 -t Renamed` | title changed; `due` + `notes` + body untouched |
| 2 | `update TASK-1 -U due` | `due` key gone; `title` + `notes` survive |
| 3 | `update TASK-1 -P due=2026-12-31` | re-set; others survive |
| 4 | `update TASK-1 --clear-body` | body empty, frontmatter intact |
| 5 | `update TASK-1 -P notes=` | `notes: ""` — **old semantics preserved** |
| 6 | `update TASK-1 --clear-body -b x` | exit 1, "cannot be combined" |

Case 5 is the backward-compat one: `-P key=` still sets the empty string rather
than removing, so existing scripts are unaffected; removal is the new `-U`.

*(Note during verification: I initially read a hand-created `entities/task/`
directory and concluded the writes were failing. The store uses the plural
`entities/tasks/`. The writes had been correct all along — corrected by locating
the real file rather than by changing code.)*

**AC verification:**

| AC | Status | Evidence |
|---|---|---|
| 1 pipeline fires | PASS | `TestPatchEntity_RunsFullPipeline` + mutation test |
| 2 inverse test | PASS | `TestPatchEntity_PreservesUnnamedProperties` |
| 3 set/unset/absent | PASS | `TestPatchEntity_SetUnsetAbsent` (5 cases) |
| 4 body tri-state | PASS | `TestPatchEntity_BodyTriState` (3 cases) |
| 5 `WritePrepStore` deleted | PASS | 0 refs; `luaUpdateEntity` holds 0 store refs |
| 6 mcp/cli unchanged | PASS | existing suites green; nil-delete + `-P key=` pinned |
| 7 CLI gate is no-op | PASS | manual case 1–5; `AllowAllFieldGate` wired |
| 8 CI gates | PASS | see below |
| 9 locked refused | PASS | `TestPatchEntity_LockedEntityRefused` |
| 10 elevation total | PASS | `TestPatchEntity_ElevationSkipsFieldGate` |
| 11 automation ungated | PASS | `TestPatchEntity_AutomationNotFieldGated` |
| RR-32XA5V ordering | PASS | `TestPatchEntity_GateRunsAfterAuthorize` (2 cases) |

**Gates:** `go test ./...` 0 failures · `just lint` 0 issues · `just arch-lint`
OK · `just lint-md` 0 issues · `just coverage-check` PASS (77.0%, up from 76.9%)
· `go test -race` on all five touched packages clean.

arch-lint passing **confirms the design bet**: declaring `FieldWriteGate` as a
consumer-side interface inside `entitymanager` needed no `.go-arch-lint.yml`
change, exactly as planned.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed rather than invented: `updateCore` mirrors `createCore`
(free-standing shared core, no ACL/attribution); `entity.Patch` mirrors
`entity.RelationOptions` field-for-field (`Properties`/`MetaUnset`/`Content
*string`); `AllowAllFieldGate` mirrors `acl.NopACL{}` as a named opt-out;
`PatchEntity`'s read-then-authorize mirrors `DeleteEntity`.

DRY: `entity.Patch.Apply` is the single merge implementation — the four
divergent hand-rolled merges are gone. The two lua/mcp test fakes implement real
read-merge-save semantics rather than permissive stubs, so binding tests still
exercise the property that matters.

Security: the ordering test is the regression net for RR-32XA5V; the guard tests
deleted from `dataentry`/`appbuild` were **replaced** with equivalents pinning
the inverted invariant (no raw handle, elevation opt-in), not dropped.
