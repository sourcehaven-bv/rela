---
id: PLAN-T2QFCBJ
type: planning-checklist
title: 'Planning: Bound the remaining Lua read bindings'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: `rela.get_relations`, `admin.get_relations`, `admin.list_entities` and
`rela.md.entity_refs` get a bound, and their `request-path` entries leave
`unpagedread_allowlist_test.go`.

Out: a `cursor` option (DEC-IYHLNF stage 2) on any binding; a store pushdown of
the relation peer gate; `rela.list_entities` (already `bounded`).

**Acceptance Criteria:**
1. `rela.get_relations` and `admin.get_relations` take a `limit` option, default
and ceiling `maxReadLimit`, and read the store through `ListRelationsPage`.
Test: limit and clamp tests; scale-free budget test.
2. `rela.get_relations` raises on a store error instead of returning a short
list (TKT-FVQ4). Test: failing reader.
3. `admin.list_entities` takes `limit` and reads one `ListEntitiesPage`.
Test: limit test; scale-free budget test.
4. `rela.md.entity_refs` takes `ids` or `content` and reads only those ids.
Without them it reads at most `limit` rows across the types. Test: ids and
content tests, ACL gating test, scale-free budget test.
5. The four `request-path` entries are gone (`TestNoNewUnpagedReads`).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: DEC-IYHLNF already decides the shape)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal API)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `readopts.go` `parseReadOpts` / `maxReadLimit` (DEC-IYHLNF).
- `store.RelationReader.ListRelationsPage` / `ListEntitiesPage` keyset pages.
- `mentions.All` extracts code-span ids from markdown, the same rule
`resolve_refs` matches.
- `visibility.ScriptReader.ListEntityPage` (TKT-D9UO9Y4, open PR #2248) is
header-only and offset based; it does not fit a binding that returns bodies.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `lua.EntityReader` gains `ListRelationsPage`. `visibility.ScriptReader`
implements it as a gated page: raw keyset pages sized to the rows still wanted,
peer-gated with `FilterRelations`, until the page is full or the store is
exhausted. A full page ends on a visible row, so its cursor never encodes a
hidden relation key. `UnrestrictedReader` passes through, `DenyReader` refuses.
- `lua.ElevatedReader` = `EntityReader` + `ListEntitiesPage`, the type of
`WriteDeps.ElevatedReader`. `UnrestrictedReader` passes through.
- `relationQuery` reads a `limit` option with the shared limit parser.
- `entity_refs`: `ids` / `content` select a type-less `EntityQuery{IDs}`; types
filter in Go. Legacy form stops at `limit` rows (recategorized `bounded`).
- `composeReadScope` declines pushdown when the query names ids: GraphQuery has
no id filter, so a pushed query would drop them.

Alternatives rejected: a cursor now (needs all bindings at once, DEC stage 2);
raising on the legacy `entity_refs()` (breaks documented usage).

**Files to modify:**
`internal/lua/{deps,readopts,runtime,elevation,markdown}.go`,
`internal/visibility/{luareader,unrestricted,denyreader,pushdown}.go`,
`internal/dataentry/document.go`, `internal/script/luascriptrunner.go`,
`internal/archguard/unpagedread_allowlist_test.go`, test fakes, Lua guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** script options (`limit`, `ids`, `content`):
limit must be a positive whole number, clamped to the ceiling; ids must be
strings and at most the ceiling; unknown option keys on the new surfaces raise.

**Security-Sensitive Operations:** the gated relation page keeps the peer gate
in `visibility`; its cursor never follows a hidden row. `entity_refs` with ids
still reads through the gated reader, so a hidden id is absent from the map.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria; budget tests use
`storetest.Counting` over memstore with `AssertScaleFree`.

**Edge Cases:** limit above ceiling clamps; limit 0/negative/non-integer raises;
hidden relations interleaved with visible ones still fill the page; empty `ids`;
duplicate ids; ids of a type outside `types`.

**Negative Tests:** bad options raise; store errors raise; too many ids raise.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** a script that read more than 2000 edges now gets 2000 (DEC-IYHLNF
accepts this; cursor follow-up). Dense hidden edges cost extra page reads for a
gated caller (follow-up: relation gate pushdown). Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-lua-scripting (docs-project) for the new options
and bounds.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** self-review against DEC-IYHLNF and the
collection-reads rule; no findings recorded. Code-level review runs at review.
