---
id: IMPL-0OFXSV
type: implementation-checklist
title: 'Implementation: ACL-gating test for rela.md.entity_refs (TKT-PUJNS0)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units) — the test runs a real Lua runtime against real `acl.Declarative` + `affordances.PolicyResolver` over a memstore, asserting on what the script itself emits
- [x] ~~Happy path implemented~~ (N/A: test-only change; no production code touched)
- [x] Edge cases from planning handled — both failure directions covered, see below
- [x] ~~Error handling in place~~ (N/A: test-only change)

## Test Quality

- [x] Using fixture builders or factories for test data — reuses the existing `newACLWorld` fixture rather than duplicating a policy
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario
- [x] Edge cases manually verified

**Verification Evidence:**

The point of this ticket is that the previous fix lacked a test, so the test
itself had to be proven capable of failing. **Both** directions were
mutation-verified:

- Restoring `ctx := context.Background()` (the original RR-ZA452J defect) → FAILS with `refs=` empty and the diagnostic "the binding resolved no principal and fell closed for everyone".
- Routing the listing through the raw `WritePrepStore` instead of the gated reader → FAILS with `refs=P-1,SEC-1,TKT-1` and "a hidden entity leaked into the ref map".

Both mutations reverted; `git diff internal/lua/markdown.go` is empty,
confirming no production code changed. Full sweep green (all packages except the
env-dependent docscapture); `just arch-lint` OK; golangci-lint 0 issues;
`internal/visibility` coverage holds at 91.7%.

**Why both halves matter:** asserting only "hidden entity absent" would pass
against the original bug, which returned an empty map for everyone — every
entity absent, including the readable ones. The first assertion is what actually
pins the regression.

## Quality

- [x] Code follows project patterns (mirrors the surrounding `TestScriptReads_*` guards)
- [x] Checked for DRY opportunities — reuses `newACLWorld` + `runAsAlice` instead of a new fixture
- [x] No security issues introduced
- [x] No silent failures
- [x] No debug code left behind
