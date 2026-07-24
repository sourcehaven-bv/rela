---
id: IMPL-DSV4OI
type: implementation-checklist
title: 'Implementation: lua: ReadDeps reads through visibility.Reader + visible tracer; scheduler jobs get explicit AllowAllReader; prove one role-scoped job'
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

- [x] Feature manually tested end-to-end — the ACL read tests ARE the end-to-end exercise: real `acl.Declarative` + `affordances.PolicyResolver` + memstore + a real Lua runtime, asserting on values the script itself emits (what would reach a prompt), not on internal state.
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- New tests, all passing: `TestScriptReads_{HiddenFieldRedacted, HiddenEntityInvisible, RelationsPeerGated, TraceGated, UpdatePreservesHiddenProperties, NilReaderDenies}`; `TestStampTaskAuditContext_{RunAsOverridesIdentity, EmptyRunAsKeepsSystemUser}`.
- **Mutation-verified, not assumed**: repointing `VisibleReader` at the raw store fails 4 of the read tests; bypassing the tracer decorator fails the 5th. Both mutations reverted and re-verified green. This rules out vacuous assertions.
- Full sweep green (`go test` over every package except the env-dependent docscapture) — every pre-existing Lua/dataentry/scheduler test passes unchanged, which is the NopACL byte-parity criterion.
- golangci-lint 0 issues; `just arch-lint` OK; `just plimsoll` OK; gofmt clean; `go build ./...` OK.
- Compiler-driven wiring audit: renaming `ReadDeps.Store` forced every construction site to be revisited (5 struct literals + the appbuild helpers), each now carrying an explicit, commented read-posture choice.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — `Runtime.reader()` is one choke point for six call sites; `scriptEntityReader`/`scriptTracer` are shared helpers rather than duplicated per wiring site
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned; construction faults degrade with a loud warning, denials are real denials)
- [x] No debug code left behind

**Deviations from plan, all deliberate and documented:**
- `appbuild` has no affordance resolver → its readers use `NopRedactor`, giving scheduler/cascade paths ROW gating without field redaction. Data-entry (which has a resolver) gets both. Weaker but never wrong; commented at the call site.
- Two plimsoll ceilings bumped by exactly the methods added, each with rationale; the two helper variants were unexported specifically to keep `Services` at +1 rather than +3.
- `visibility.mayDependOn` regained `store` — RR-RT5YV8 removed it in PR 1 when only `EntityGetter` was needed; `ScriptReader` now uses `store.EntityQuery` concretely, so the allowance is earned rather than speculative.
- AC10/AC12 split to TKT-76JP2A (user decision) — see the plan's scope note.
