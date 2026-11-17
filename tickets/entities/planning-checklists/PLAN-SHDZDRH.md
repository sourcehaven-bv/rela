---
id: PLAN-SHDZDRH
type: planning-checklist
title: 'Planning: Archguard allowlist for unpaged list reads'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: an archguard test pinning unpaged whole-collection reads in
non-test code outside `internal/store`, per function, with a category and
reason; the rule doc pointer; the FED6FMM summary. Out: removing any read
(TKT-FED6FMM).

**Acceptance Criteria:**
1. The test passes on the branch: `go test ./internal/archguard/`.
2. A new unpaged call fails it: `TestScanUnpagedReads_FindsSyntheticRead`.
3. A stale entry fails it: `diffAllowlist` reports a missing key (`TestDiffAllowlist`).
4. Every entry has a category and reason: `TestUnpagedReadAllowlist_HasReasons`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small guard following an existing pattern)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: go/ast in the standard library)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo pattern exists)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: archguard pattern is the prior art)

**Research Doc:** N/A

**Existing Solutions:** `internal/archguard/bareref_test.go`,
`identityedge_test.go` and `guard_test.go` (`scanTree`, `diffAllowlist`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Syntactic go/ast matcher on method name (any receiver,
so interface calls count). Unpaged: ListEntities/ListRelations/ListEntityHeaders
always; *Page and GraphQuery/GraphQueryHeaders when Limit is unset. Excluded:
queries keyed by ids (IDs, EntityIDs, EntityID, From, To, PeerIDs) and reader
decorators forwarding their own query parameter. Keyed per function so a
category fits each entry.

Alternatives: go/types (rejected: needs a full type-check of the module per test
run; the other guards are syntactic); per-file keys like bareref (rejected:
files mix batch and request reads).

**Files to modify:** internal/archguard/unpagedread_test.go,
internal/archguard/unpagedread_allowlist_test.go,
.claude/rules/collection-reads.md

## Security Considerations

- [x] ~~Input sources identified~~ (N/A: test-only code reading the repo's own sources)
- [x] ~~Input validation approach defined~~ (N/A: test-only)
- [x] ~~Security-sensitive operations identified~~ (N/A: test-only)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: test-only)

**Input Sources & Validation:** N/A

**Security-Sensitive Operations:** N/A

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Table test of the matcher; synthetic tree scan end to end;
the real tree against the allowlist.

**Edge Cases:** Pointer literal queries, variables with later `q.IDs =`, generic
receivers, closures, GraphQuery composite literals that are not calls,
forwarding decorators.

**Negative Tests:** Unlisted function, over count, under count and stale entries
fail via `diffAllowlist`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] ~~Security risks assessed~~ (N/A: test-only)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Heuristic misses (query keyed through a helper, pass-through helpers
hiding a caller's scan) are documented in the matcher's doc comment and recorded
as `bounded`/`pass-through` entries. Effort: s.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal guard)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: no user-facing docs)

**Documentation Impact:**
- [x] N/A - Internal change; `.claude/rules/collection-reads.md` gets a pointer.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small test-only guard; reviewed by cranky-code-reviewer in review)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: see above)

**Design Review Findings:** N/A
