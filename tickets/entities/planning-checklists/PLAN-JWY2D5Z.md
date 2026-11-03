---
id: PLAN-JWY2D5Z
type: planning-checklist
title: 'Planning: Mentions follow a rename: tombstones on database stores, in-place rewrite on fs'
started: "2026-11-02"
completed: "2026-11-02"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: after `RenameFamily` (plain rename and conversion), a code-span
mention of the old id resolves to the new entity on every backend, and counts as
a backlink of it. Out: reversing a conversion; redirecting old URLs and API GETs
by old id.

**Acceptance Criteria:**
1. fs and mem: a rename rewrites `` `OLD` `` to `` `NEW` `` in every live body that mentions it, and emits an update event per rewritten entity. Test: `storetest ` mention suite on fs and mem.
2. sqlite and postgres: a rename records OLD→NEW in `retired_ids `; `Successors ` returns it; text is not rewritten. Test: `RunRetiredIDTests `.
3. A retired id is reserved: create and rename-onto it return `ErrConflict `; `HighestID ` counts it. Test: `RetiredIDIsReserved `, `HighestIDCountsRetiredIDs `.
4. Chains collapse (A→B→C leaves A→C, B→C); renaming back reclaims the own id. Test: `ChainsCollapse `, `RenameBackReclaimsOwnID `.
5. `MentionSources(NEW) ` includes bodies that mention OLD. Test: storetest mention suite.
6. Read path: a retired mention resolves only when the reader can read the successor. Test: `TestResolver_ResolveMentions `.
7. SPA links a retired mention to the new id. Test: vitest for `markdown.ts `, `entityRefResolver.ts `, `entityRefResolution.ts `.

## Research

- [x] ~~For larger features: run `/research `~~ (N/A: approach settled in discussion with the user before filing)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: storage-internal change)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: storage-internal change)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `store.MentionIndex ` and `internal/mentions ` parser are reused for the fs/mem rewrite.
- The soft-delete id reservation (`markedIDTaken `) is the pattern for reserving retired ids.
- Optional store capabilities (`HistoryReader `, `VersionPurger `) are the pattern for `store.RetiredIDs `.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Optional capability `store.RetiredIDs.Successors(ctx,
ids) `. sqlite (schema v31) and postgres (migration 0038) add a
`retired_ids(id, successor, retired_at) ` table written inside `RenameFamily
`. `visibility.Resolver.ResolveMentions ` follows successors through the same
row gate. The wire `Mention ` gains `id `; the SPA links to it.

Deviation from the ticket: the tombstone stores no old type, face or cause.
Nothing reads them yet; URL redirects (out of scope) can add them.

Alternatives rejected: rewriting text on database stores (the user chose
"resolve only"); a tombstone on fs (fs is VCS-backed, rewrite is enough).

**Files to modify:** `internal/store/{store.go,mentions.go} `,
`internal/store/{fsstore,memstore,sqlitestore,pgstore}/* `,
`internal/sqlitedb/* `, `internal/store/storetest/* `,
`internal/visibility/mentions.go `, `internal/dataentry/mentions.go `,
`internal/apiwire/v1/responses.go `, `internal/archguard/mentionindex_test.go
`, frontend markdown/mention resolution, GUIDE-data-entry.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Ids come from the existing rename path, which
already validates them. Mention ids come from parsed bodies and are looked up
with parameterized SQL.

**Security-Sensitive Operations:** Successor lookup on the read path: the
successor is resolved through `ResolveIDsErr `, the same row gate as any
mention, so an unreadable successor is dropped. `Successors ` is limited to
store and visibility by archguard. The fs/mem rewrite runs below the ACL like
the relation re-key; it replaces an id and discloses nothing.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria.

**Edge Cases:**
- Case-only rename retires nothing.
- Delete of the successor keeps the reservation.
- Rename inside `Tx `.
- Code span inside link text is not rewritten.

**Negative Tests:** Create or rename onto a retired id fails with `ErrConflict
`; an unreadable successor is not resolved; a store without the capability does
not resolve retired ids.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Existing tests reuse an id after a rename: adapted per backend capability.
- Concurrent create of the old id during a rename on postgres: `lockRenamePair ` takes both family locks in sorted order. A create that differs only in case is not covered.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md (via GUIDE-data-entry): "Mentions after a rename"

## Design Review

- [x] ~~Run `/design-review ` before starting implementation~~ (N/A: design settled with the user in conversation before implementation; `/code-review ` covers the result)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
