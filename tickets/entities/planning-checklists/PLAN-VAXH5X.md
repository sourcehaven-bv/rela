---
id: PLAN-VAXH5X
type: planning-checklist
title: 'Planning: Guard test forbids zero-face reads outside a shrinking allowlist; faced fixtures by default'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a go/ast guard test that pins every non-test call to a
two-argument `.GetEntity(`, to `.getEntity(` and to `bareEntityID(` under
`internal/` and `cmd/` in a per-file exact-count allowlist; retiring
`seedDraftAndPublishedTicket` in `internal/dataentry` tests in favour of a type
that declares faces with rows only at declared faces. Out: removing any existing
call (stages 1 and 2); any production behaviour change; fixing face-blind
surfaces (tests that only passed on the legacy row are skipped with the backlog
bug id).

**Acceptance Criteria:**
1. A new two-argument `GetEntity(ctx, id)` call in a file not on the allowlist fails the guard. Test: run the checker over a synthetic source snippet and assert a violation.
2. A file whose count rises fails; a file whose count falls fails (the list stays exact). Test: checker compared against a synthetic allowlist that is one over and one under.
3. The real tree matches the allowlist exactly. Test: the guard over `internal/` and `cmd/`.
4. No `internal/dataentry` test seeds a zero-face row for a type that declares faces; `seedDraftAndPublishedTicket` is deleted. Test: the converted tests pass or skip with a bug id.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-Y6JA37 (Stage 0), DEC-NPZICR.

**Existing Solutions:**
- `golang.org/x/tools/go/packages` is not a dependency (only stale go.sum entries). Type-checking the whole tree from a unit test would also need `go list -export` and would miss build-tagged backend files. Rejected in favour of a syntactic heuristic.
- `internal/dataentry/world_test.go` `TestWorldCapableRoutesDoNotUseUngatedReader` (go/ast walk) and `internal/acl/ceilingguard_test.go` (exemption list with reasons) are the precedents.
- `facedApp`/`seedDeclaredFaceTicket` in `internal/dataentry` are the realistic fixtures.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** New test-only package `internal/archguard` (only
`_test.go` files, so arch-lint and coverage see nothing). `zeroface_test.go`:
- `countZeroFaceReads(file) int` counts a `.GetEntity` or `.getEntity` selector called with two args or used as a method value, a `.GetEntityState` call whose face argument is the literal `""`, and any non-declaration reference to `bareEntityID`.
- Heuristic, documented: without type information the checker cannot tell a `store.Store` receiver from a consumer-side interface or an address-taking visibility reader, so it counts every two-argument `GetEntity` call. That over-counts (address-taking readers such as `visibility.ScriptReader` are counted), which is safe: the list only shrinks, and a stage that moves a call onto an address-taking API removes it from the count. Three-argument `GetEntity` (the `cli/sync` HTTP client) is excluded by arity.
- `zeroFaceAllowlist map[string]int` (repo-relative path to count) in its own file, with a comment: may only shrink, names the alternatives (`store.GetEntityState`, `store.GetEntityAt`, `visibleReader.getVisibleRef`, `getEntityRef`, the visibility readers) and DEC-NPZICR.
- Excluded directories: `internal/store/storetest` and `internal/visibility/visibilitytest` (conformance harnesses that test the store's zero-face contract itself; they change with the API in stage 2). Also skips `testdata`, `node_modules`, `_test.go`.
- `diffAllowlist(got, want) []string` returns one message per file over, under, or new; the test fails with all messages.
- Synthetic test: parse in-memory snippets with a violation, a three-arg call and a `GetEntityState` call, assert the count; diff against over, under and new allowlists.
Fixtures: add a ticket app whose ticket declares `draft`, `published`, `review`,
seeded only at declared faces; move the four callers; tests that only passed on
the zero-face row get `t.Skip("face-blind: BUG-xxx")`; delete
`seedDraftAndPublishedTicket`.

**Files to modify:**
- `internal/archguard/zeroface_test.go` (new)
- `internal/archguard/zeroface_allowlist_test.go` (new)
- `internal/dataentry/facegate_surfaces_test.go`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Only the repository's own Go source, read by a
test. No runtime input.

**Security-Sensitive Operations:** None at runtime. The guard supports security
by stopping new reads that skip the face gate. The fixture change removes a
legacy row that let face-gate tests pass for the wrong reason; skipped tests are
tracked by their bug ids.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1/AC2: checker tests over synthetic source and allowlists.
AC3: the guard over the real tree. AC4: converted dataentry tests.

**Edge Cases:**
- Method value without call (`s.GetEntity` passed as a func value): not a call, not counted; documented.
- Three-argument `GetEntity`: not counted.
- Allowlisted file deleted or renamed: reported as under (count 0).
- Build-tagged files (pgstore, sqlitestore): parsed regardless of tags, so they are counted.

**Negative Tests:** New file with a call, file over count, file under count:
each fails with a message naming DEC-NPZICR and the alternatives.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Parallel branches (BUG-1YN750, BUG-8J3LSB) change counts: the list is exact, so rebases must update it; the failure message says how.
- The heuristic over-counts address-taking reads: accepted and documented.
- Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] N/A - Internal change, no user-facing docs needed. The allowlist comment documents the rule.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-8TG4HL (significant: also count
`GetEntityState(ctx, id, "")`), RR-HSD3CR (significant: count method values, not
only calls), RR-BGQYUA (minor: arity filter on `getEntity`). All addressed in
the approach above.
