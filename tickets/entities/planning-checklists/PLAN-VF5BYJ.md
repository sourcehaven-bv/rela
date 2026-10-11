---
id: PLAN-VF5BYJ
type: planning-checklist
title: 'Planning: External-ref property type and Lua 3-way merge helper'
started: "2026-10-08"
completed: "2026-10-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: external_ref property type, PropKeyEqual store predicate, rela.find_by_external_ref, rela.sync.merge (internal/syncmerge), expect/token on Lua create/update, startup refusal without version history, read-only SPA widget, docs. Out: OAuth and Basecamp connector (TKT-01KZSO), hand-editing refs, automatic conflict policies, search indexing of ref ids.

**Acceptance Criteria:** AC1-14 in the Plan below, as revised by D1-D12.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach decided with the user in session; codebase surveyed by Explore agents)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: merge over rela property types; no library models them)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** unique: check in writeWithUniqueCheck, derived pg indexes, rejectComputedPatch, version tags (TKT-VO6VG9) as the base pointer, 3-way merge as in git.
<!-- Document what you found:
- Libraries considered (with pros/cons, why chosen or rejected)
- Similar patterns in codebase (file:line references)
- Reference implementations that inspired the approach
- Relevant concepts from rela-docs or rela-issues-and-design-tickets
-->

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** See Plan > Design.

**Files to modify:** See Plan > Files.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Ref values from Lua scripts, migrations and imports only (D3); id allowlist (printable, no control or format characters, <= 256 bytes), url http/https with host; theirs values coerced per type or raise.

**Security-Sensitive Operations:** Ref lookup gated and re-checked on the redacted row; unique 422 withholds the other entity; base moves need update plus tag:sync; ref writes refused on interactive surfaces.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See Plan > Test plan.

**Edge Cases:** Faces, soft-deleted holders, hidden holders, unreported fields, explicit clears (EMPTY), CRLF content, numeric ids, Unicode format characters, concurrent duplicate on pg.
<!-- List specific edge cases and expected behavior. Consider:
- Empty/null/missing values
- Boundary values (0, -1, MAX_INT)
- Special characters, unicode, null bytes
- Concurrent access
- Resource exhaustion
-->

**Negative Tests:** Invalid schema combos, invalid values (422), unknown operator (ErrInvalidQuery), stale expect (nil, "conflict"), hidden synced field raises, fs/mem with sync refs refused at host startup.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** See Plan > Risks. Effort: l.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [x] docs/metamodel.md - New metamodel features
- [x] docs/cli-reference.md - New/changed commands
- [x] docs/data-entry.md - UI changes
- [x] CLAUDE.md - New patterns or conventions
- [x] README.md - Project-level changes
- [x] N/A - Internal change, no user-facing docs needed
-->

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-8HRIYY, RR-BGLH9Q, RR-JAV2WN, RR-12DOS0, RR-6GGVGS, RR-B3I6WR, RR-D26OC0, RR-LFPDGB, RR-2E3J33, RR-92RSF1, RR-SMQ882 (all addressed as D1-D12).

## Plan


## Problem
A sync connector (FEAT-XYQMUB; next TKT-01KZSO Basecamp) needs: a typed, unique
link from an entity to its counterpart elsewhere; a fast lookup by that link
(webhooks carry an external id); a correct 3-way merge against the base tagged
`sync/<system>`; a guarantee that sync never runs on a backend without history.
The merge must converge (echoes stop) and must not lose a user edit made while a
sync runs.

## Scope
In:
1. `external_ref` property type: schema, validation, storage on all backends,
   uniqueness per system, OpenAPI.
2. Narrow store extension `PropPredicate.Key` so ref lookups push down on pg/sqlite.
3. Lua `rela.find_by_external_ref(system, id)`.
4. Lua `rela.sync.merge(base, ours, theirs, opts)`, pure, backed by pure Go `internal/syncmerge`.
5. Lua `rela.update_entity`/`rela.create_entity`: `opts.expect` and a third return
   value, the token of the row as written (closes the write/tag race).
6. A schema with `sync: true` refs fails to assemble without a version tagger (fs, mem).
7. Minimal frontend: read-only display widget and cell format. CLI and MCP display.
8. Docs: GUIDE-metamodel, GUIDE-lua-scripting (sync section, reference loop).

Out: OAuth binding and Basecamp connector (TKT-01KZSO); hand-editing refs in forms
or via CLI `-P`; automatic conflict policies; search indexing of ref ids;
predicate/filter access to `entity.ref.id`; per-system tag permissions.

## Design

### Data model
One declared property per system; the system lives in the schema:
```yaml
ticket:
  properties:
    basecamp:
      type: external_ref
      system: basecamp   # required; tag-segment grammar
      sync: true         # optional; requires a versioning backend
```
Stored value is an object: `basecamp: {id: "7012345678", url: "https://..."}`.
Map values already round-trip on every backend and through Lua; no storage
migration. No version number in the value; the base is the `sync/<system>` tag.

Rejected: separate table (no fs/history/audit/ACL); one list property of
`{system,id,url}` (field grants are per property; uniqueness and lookup become
containment); plain string id (Basecamp URLs carry bucket ids).

### Validation
Loader: `system` required, valid as `sync/<system>` tag name; `sync`/`system` only
on external_ref; not combinable with list, unique, default, computed, values,
format, required; not on relation properties; one ref property per system per
type; not a display_property; `sync: true` refused on faced types.
Value: map with string keys; keys `id` (required) and `url` (optional) only; id a
non-empty string <= 256 bytes, no control chars, numbers rejected ("quote numeric
ids"); url http/https with host. Register in IsBuiltinType, isStringValuedType
(false), predicatefns (unmodelled; compile error), widget `external-ref`, OpenAPI
object schema.

### Uniqueness
(system, id) unique across all types declaring the system, per face.
`uniqueValues` emits ref entries so writes go through `writeWithUniqueCheck`'s
serialized Tx; `checkExternalRefUnique` runs one `GraphQueryHeaders` per
(type, prop) with a Key predicate, all faces, excluding self. Soft-deleted holders
keep the id. The 422 names the property only. pg backstop: derived partial unique
index per (type, prop) on `(type, properties->'p'->>'id', face)`. History restore
already runs the check.

### Lookup
`PropPredicate.Key`: only with PropEqual, Scalar, non-empty Value; else
ErrInvalidQuery. pg jsonb path with typeof string; sqlite nested json path via
builder helpers; naive map lookup. Conformance + RunGraphDifferential + pg EXPLAIN.
Visibility `FindByExternalRef` on ScriptReader/UnrestrictedReader: raw header
query per (type, prop), dedupe, `GetAddress` in the reader's world, keep only if
the redacted row still carries the id (no oracle). Lua returns entity or nil;
raises on unknown system or ambiguity (>1 entity).

### Merge helper
`local r = rela.sync.merge(base, ours, theirs, {fields = {...}})`
- base: `rela.version_by_tag(addr, "sync/"..system)` or nil; ours:
  `rela.get_entity`; theirs: `{properties = {...}, content = ...}` mapped by the
  connector. `"content"` means the body (refused if the type has a `content`
  property); undeclared field raises.
- Returns `{base_unknown, write, push, conflicts = {{field, base, ours, theirs}}, unchanged}`.
- Per field: eq(ours,theirs) unchanged; eq(base,ours) write theirs;
  eq(base,theirs) push ours; else conflict. base nil: base_unknown, no write/push,
  differing fields are conflicts.
- A synced field in `ours.redacted`/`base.redacted` raises.
- Equality per type: empty values equal; strings exact; date by parsed date;
  datetime by instant; integer numeric; boolean; list as multiset; ref by id;
  content CRLF->LF, trailing whitespace trimmed. `write` values canonicalized.

### Race-free tag moves
write -> version_token -> tag can tag a concurrent user edit as base. Fix:
`update_entity(id, props, content, {expect = tok})` uses the per-reader CAS (as
TagCurrent) and returns `nil, "conflict"` on mismatch; success returns the token
of the caller's view of the row as written (from the write result, not a re-read).
`create_entity` returns the token too. Reference loop:
```lua
local e, _, tok = rela.update_entity(id, r.write, nil, {expect = tok0})
if e then rela.tag_version(id, "sync/basecamp", {expect = tok}) end
```
Push path: token before the HTTP call, `tag_version{expect}` after.

### Backend refusal
`metamodel.SyncRefProps(m)`; in `appbuild.assemble` after `versionTaggerFor`: if
non-empty and no tagger, fail "schema declares sync-managed external refs (...);
sync needs the SQLite or PostgreSQL build". Non-sync refs work on fs.

### Security and ACL
Writes via field write grants (operator grants refs to `system:basecamp`); reads
via `visible:`; lookup re-checks the redacted read; unique 422 withholds the other
entity; base moves need update + `tag:sync` (covers all systems); URL scheme
allowlisted and SPA links only http(s); merge helper has no I/O.

### Frontend, CLI, MCP
Read-only ExternalRefWidget (id linked to url, system label), cell shows id, never
sent in a PATCH. `rela show` prints `id <url>`; JSON and MCP show the object.

## Acceptance criteria
1. Valid schema loads; each invalid combination fails with a specific message.
2. `{id,url}` round-trips through create/get on mem, fs, sqlite, pg.
3. Non-string id, missing id, unknown key, javascript: url, list value: 422.
4. No shared (system,id) on one face across or within types; faces of one entity
   may share; soft-deleted holder keeps it; error does not name the other entity.
5. pg concurrent duplicate within a type rejected by the index, same 422.
6. Key predicate identical on naive/sqlite/pg; invalid combos ErrInvalidQuery; pg
   uses the index; sqlite one query.
7. find_by_external_ref: entity; nil for unknown, hidden row or hidden ref; raises
   on unknown system and ambiguity.
8. merge classification; base nil behavior.
9. Equality rules; canonical write values.
10. Hidden synced field raises; undeclared field raises; content clash refused.
11. update_entity stale expect: nil,"conflict", no write; success token lets the
    tag succeed; another principal's write in between makes the tag conflict.
12. fs/mem with sync refs fail to assemble; sqlite/pg start; non-sync ref on fs starts.
13. Simulated pull/push loop on sqlite reaches a fixed point within 2 rounds.
14. SPA shows the ref as a link, never PATCHes it, no link for non-http url.

## Test plan
1 loader tests; 3 validation tests; 2 storetest round-trip; 4 entitymanager
unique tests; 5 pg derivedschema + concurrency; 6 storetest graphquery, graphdiff,
pg/sqlite EXPLAIN; 7 lua + visibility tests with redaction; 8-10 syncmerge table
+ fuzz (swap symmetry), lua table shape; 11 sqlite lua interleaving; 12 appbuild
per tag; 13 sqlite end-to-end with stub external map and background automation;
14 vitest.

## Files
metamodel (types, loader, validation, schema_output, new externalref.go);
predicatefns/env.go; openapi/schemas.go; store graphquery + naive + pg/sqlite +
storetest; appbuild (assemble, derived schema); entitymanager/unique.go;
visibility luareader/unrestricted; new internal/syncmerge; lua sync.go and
runtime update/create; cli show; frontend types, registry, ExternalRefWidget,
format, DynamicForm; guides.

## Risks
Cross-type uniqueness has no pg index (relies on the serialized Tx; pinned by a
test). Store contract grows (narrow, differential-tested). Faced types refused for
sync. `tag:sync` shared across systems. Purged current state: connector treats as
base unknown. Echo push before the pull's tag is harmless. First map-valued
property: audit `case []any` sites (CSV export, history diff).

## Effort
L (5-7 days).

## Decisions (user, 2026-10-08)
1. One property per system.
2. (system, id) unique across all types.
3. `sync: true` is an explicit opt-in.
4. The race fix (`expect` on update/create plus returned token) lands in this ticket.
5. Lists compare as multisets (default, not objected to).

## Design review revisions (binding; they override the sections above)

D1 (critical) Base moves only when `conflicts` is empty. When every field is
unchanged but ours differs from base (both sides converged), tag the current
state with the read token. Guide states both; syncmerge tests and AC13 cover both.

D2 (critical) Write-returned token = per-reader token of the stored row as
decoded (pg RETURNING / decode, sqlite re-select inside the write), redacted
through the caller's reader. Never VersionOf of the in-memory input (int64 vs
int `%T` mismatch). AC11 on pg and sqlite with integer, float, date, map values:
write token == token of a fresh `version_token` read.

D3 (critical, security) External-ref values are not writable from interactive
surfaces: data-entry PATCH/POST, MCP writes, CLI `-P`, forms (like
`rejectComputedPatch`). Writable only from Lua script writes (operator-authored),
data migrations and imports. Field write grants still apply on top when
configured. Restore (history_restore) keeps the current ref values. Tests.

D4 Caller-view CAS for update_entity is a new expectation (CallerView), not raw
`Patch.ExpectedVersion`: patchEntityOnce reads raw then view, compares the view
token, passes VersionOf(raw) to the store; no patchWithRetry on this path. The
reference loop re-reads and re-merges with a fixed bound (3) when the tag
conflicts (cascades/background jobs may rewrite after the write) and logs when
it gives up.

D5 A nil lookup does not mean the id is free (soft-deleted or hidden holder,
raw-path duplicates). A unique 422 on create is terminal for that item in the
reference connector. Ambiguity is counted after the visibility filter. Document
that the connector principal must read every type declaring the system. Tests
for soft-deleted and hidden holders.

D6 A field absent from theirs is "not reported" and skipped; a real clear is
`rela.sync.EMPTY`. Theirs values that cannot be coerced to the property type
(non-numeric integer, enum not in values) raise per field.

D7 Lookup uses a distinct operator `PropKeyEqual` (with Key) so switches that do
not know it fail closed; queryplan, listpushdown and derived-index inference
reject it explicitly.

D8 `system` joins ShapeProjection (not RenderProjection); a change is
needs-migration. `migrate gen` emits a Lua stub for string -> external_ref;
documented. Conflict resolution is a documented connector pattern: the connector
records the conflict (property/comment); a user-triggered action running as the
connector principal applies the chosen side, pushes, and tags.

D9 pg backstop: new DerivedObjectKind with its own name prefix, blocker count,
violation mapped back to the property, `jsonb_typeof(...->'id') = 'string'` in
the predicate. The soft-delete scan covers every type declaring the system.

D10 Refuse property-change triggers on refs at load; exports and cells format a
ref as its id; duplicate omits refs with a reason; `{}` is empty, defined once.

D11 No refusal at assemble for read-only tooling. Refuse at startup of the
long-running hosts (rela-server, desktop, scheduler) when `sync: true` refs exist
without a version tagger; `rela.sync.merge` and `find_by_external_ref` raise
"sync needs version history" at use time otherwise.

D12 Tests: AC13 stub normalizes content (whitespace) instead of echoing;
property-based convergence fuzz (any interleaving reaches a fixed point within
two rounds, no user edit lost); `rela analyze` check for duplicate
(system, id) pairs.
