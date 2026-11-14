---
id: PLAN-X84HBKW
type: planning-checklist
title: 'Planning: Remove unpaged list reads from request paths'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the `request-path` entries of
`internal/archguard/unpagedread_allowlist_test.go` that a store query already in
the `store.Store` contract can bound: id minting and `unique:` checks on create,
the dead holder-lookup fallbacks, gantt edges, `rela.principals_with_role`, and
`rela.list_entities` (already bounded by its limit). Out, filed as follow-ups:
entries that need a new gated read capability or an API change (dataentry
`scopedHeaders` and its list helpers, MCP `list_entities`, search with no text,
ingest find, rename dry run, the other Lua bindings, desktop notifications).

**Acceptance Criteria:**
1. Each fixed entry is gone from the allowlist, and `TestNoNewUnpagedReads` passes.
2. Each fix has a `storetest.AssertScaleFree` budget test: same calls and rows over 100 and 1,000 padding rows.
3. Behavior is unchanged: sequential ids continue from the highest, short ids avoid taken and soft-deleted ids, `unique:` still refuses a duplicate, gantt keeps only edges inside the node set, role holders still require a subject held by one entity.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: uses existing store queries)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal store queries)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal refactor)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `store.EntityReader.HighestID` (all backends),
`store.GraphQueryHeaders` with `PropEqual` and `Narrowing`,
`RelationQuery.EntityIDs`/`PeerIDs`, `storetest.AssertScaleFree` (TKT-A6A97WV).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- Sequential ids: `HighestID` for the prefix. Short ids: one count for the length, then batches of 10 candidates checked with one `ListEntityHeaders{IDs}`.
- `unique:`: one `PropEqual` GraphQuery per unique property; Go still judges the match.
- Gantt edges: `EntityIDs` and `PeerIDs` set to the node ids.
- Role holders: read by ids, or one GraphQuery with a narrowing branch per subject.
- No new store capability. Alternative rejected: a case-folded id lookup capability for rename, deferred to a follow-up.

**Files to modify:** `internal/entity/id.go`,
`internal/entitymanager/{core,unique}.go`, `internal/store/fsstore`,
`internal/visibility/{holders,headerquery}.go`,
`internal/dataentry/gantt_handler.go`, `internal/appbuild/idproles.go`, the
allowlist.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Unique values and IdP subjects are passed as
query parameters, never as SQL text. Empty subject sets return early, so they
never become an unfiltered read.

**Security-Sensitive Operations:** No read gate changes. The write-path reads
stay raw (entitymanager write prep). Gantt edges are still filtered against the
gated node set.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** `TestCreate_CostIsScaleFree`,
`TestGanttEdges_CostIsScaleFree`, `TestRoleHolders_CostIsScaleFree`, and the
existing behavior tests in each package.

**Edge Cases:** Soft-deleted and retired ids; lower-case prefixes; a subject
held by two users; no subjects.

**Negative Tests:** `TestCreate_UniqueLookupStillRefusesDuplicate`,
`TestCreate_FailsWhenIDScanErrors`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Id minting could collide: candidates are checked against every face
and the soft-delete marks, and the store refuses a duplicate anyway. Effort: l.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] ~~User-facing docs identified~~ (N/A: internal change, no user-facing behavior)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: internal change)

**Documentation Impact:** N/A - Internal change, no user-facing docs needed.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: uses existing store queries; covered by code review)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none
