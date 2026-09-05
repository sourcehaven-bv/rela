---
id: PLAN-4MZ8WQ
type: planning-checklist
title: 'Planning: Entity commenting stage 1: property and section anchors'
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: `internal/comments` as its own service (not the graph, not
entitymanager) with a real `comments.Store` interface, four backends
(file/postgres/sqlite/memory) and a `commentstest.RunAll` conformance suite;
four per-entity ACL permissions (`comment:read|add|update|delete`); fixed
server-written fields (`Author`, `CreatedAt`); property- and section-anchored
comments; a `comments:` metamodel block carrying policy only;
create/resolve/delete from the SPA.

Out of scope: operator-declared extra comment fields (wanted, deferred — the
config and record shapes must leave room); text-range anchors + `FormatMarkdown`
normalisation (stage 2); content-hash staleness pinning (stage 3); threading;
comment search; comment version history.

Explicitly NOT doing: no entity type (synthesised or declared) — comments never
enter `store.Store`, entitymanager, the audit log or `/_schema`; **no
versioning**; **no search indexing**; no KV blob storage; and no new
`acl.Op`/`translateVerb` verb (these are named permissions, not write verbs).

**Acceptance Criteria:**

See TKT-FIO205 "Acceptance criteria" — 7 numbered criteria, each mapped to a
test in the Test Plan below.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** <!-- Link RES-xxxx if created, or N/A for small changes -->

RES-XRYX18 (anchoring research, status done).

**Existing Solutions:**

- **`github.com/vloothuis/textanchor` v0.1.0** (MIT) — published from `margin`
  during the research. Needed in **stage 2**, not this ticket.
- **Pattern followed: `transforms:`** (`metamodel/types.go:45`,
  `metamodel/transforms.go:26`) — a top-level block that validates at load and
  *canonicalises* defaults so no consumer re-defaults. Chosen over
  `attachments:` (`types.go:36`), which has no load-time validation at all.
- **`NewAttachmentPolicy`** (`metamodel/attachments.go:94`) — precedent for
  keeping accessors OFF `Metamodel` (which carries a plimsoll directive at
  `types.go:18`). A `CommentPolicy` view type mirrors it.
- **`internal/store` + `internal/store/storetest`** — the governing precedent for
  the STORAGE shape: an interface, per-backend implementations selected by build
  tag, and a conformance suite every implementation must pass. `comments.Store`
  copies this.
- **`internal/userstate`** — the precedent for the SEPARATION argument only. Its
  package doc opens with "Why this is not in the graph" and makes exactly this
  case for snoozes. Its KV storage is deliberately NOT copied: a snooze is a
  single flag, a comment is a queryable record.
- **`acl.Request.HoldsPermissionForEntity`** (`resolver.go:260`) + the
  `statemachine` transition guard (`appbuild/transitions.go:21-39`) — the
  precedent for a per-entity named permission, including the served-vs-inert
  fail-closed rule. Copied directly.
- **`acl.PermHistoryRead`** (`policy.go:47`) — the precedent for a rela-shipped
  named permission granted via a role's `permissions:` list, and for registering
  it in `BuiltinPermissions()`.
- **`DocumentsPanel.vue`** — the model for the panel (props-in, self-gating,
  own fetch, SSE refresh).
- Rejected: synthesising an entity type; operator-declared comment type + Lua
  author stamping. Both in Approach → Alternatives rejected.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

**Comments are their own thing** — their own service, their own storage
interface with real backends, and their own ACL permissions. Not the graph, not
entitymanager, not a KV blob.

1. **`internal/comments`** — the service package. Defines the `Comment` record
   and a `comments.Store` interface (`List(ctx, target)`, `Add`, `Update`,
   `Delete`), plus `commentstest.RunAll` that every backend must pass. This is
   the `store.Store` / `internal/store/storetest` pattern (a real record store
   with a conformance suite), NOT `userstate`'s KV blob: comments are records to
   be listed, filtered and counted per target, and a blob would force
   read-modify-write on every add while making per-entity queries impossible.

2. **Backends**, selected at wiring time by build tag exactly as the entity
   store is (`appbuild_{fs,postgres,sqlite,memory}.go`):
   - `filecomments` (default) — YAML/TOML under `.rela/comments/`, one file per
     target. Human-readable and diffable, consistent with rela's file-first tier.
   - `pgcomments` — a real table in the tenant schema, indexed on target.
   - `sqlitecomments` — a table in `.rela/rela.db`.
   - `memcomments` — tests.
   Keep backend-specific imports inside the tagged recipe files; CI already
   asserts the postgres build links no bleve and the default build links no pgx.

3. **ACL: four named permissions, resolved per target entity.**
   `comment:read`, `comment:add`, `comment:update`, `comment:delete`, granted
   through a role's `permissions:` list like the `history:read` family, and added
   to `acl.BuiltinPermissions()` (`policy.go:72`) — `permguard_test.go` fails
   otherwise.

   Resolution goes through **`acl.Request.HoldsPermissionForEntity`**
   (`resolver.go:260`), the subject-aware sibling of `HoldsPermission`, so a
   permission conferred by an ownership relation to the target is honoured — the
   same seam the statemachine transition guard uses. Adapter modelled on
   `appbuild/transitions.go:21-39`, including its served-vs-inert rule: inert
   when no policy is configured at all, **fail closed** when a policy exists but
   the `acl.Request` is unexpectedly absent.

   `comment:read` is additionally **floored by the target's read verdict** — a
   principal who cannot read the entity cannot read its comments however the
   comment grants read, and cannot tell "none" from "denied".

   No new `acl.Op`, no `translateVerb` case: these are named permissions, not
   graph write verbs, so the `_actions` contract is untouched.

4. **Author stamping is trivial because rela owns the path.** The handler reads
   `principal.From(ctx)` and sets `Author`; it is never read from the request
   body. None of the in-graph routes can do this (`{{user.name}}` is the git
   user; `computed:` cannot see the principal; `admin.update_entity` does not
   exist by design) — which is precisely why the service is rela-owned.

5. **`comments:` metamodel block carries policy only** — `{enabled, on: [types]}`.
   No schema, no synthesised type. Validated at load following
   `validateTransforms` (`transforms.go:26`): sorted keys, canonicalised
   defaults, load fails on a bad block; `on:` entries must name real entity
   types. `validTopLevelKeys` (`loader.go:16`) gains `comments` in the same
   change (BUG-5XIN07).

6. **HTTP:** `/api/v1/_comments/{type}/{id}` (GET list, POST add) and
   `/api/v1/_comments/{type}/{id}/{commentID}` (PATCH, DELETE). The underscore
   prefix is the reserved-segment convention (`api_v1.go:139`). Probes required
   in `router_walk_test.go` and `readonly_write_route_invariant_test.go`.

7. **UI.** The per-property affordance attaches at `PropertyDisplay.vue:92`
   (`<dt>`) and `SectionEditForm.vue:283` — both arms are reachable from the same
   section in `EntityDetail.vue:975-996`, so both must be covered.
   `SectionEditForm` already has `entityType`/`entityId` in scope (`:66-67`);
   `PropertyDisplay` needs `entityId` threaded in. Panel modelled on
   `DocumentsPanel.vue` (props-in, self-gating, own fetch, SSE refresh).

**Deliberately not built:** no versioning (comments are not version-captured),
no search indexing, no entity type. All three were considered and rejected —
comments are commentary, not domain records.

**Alternatives rejected:**

- **Synthesising a `comment` entity type into the loaded metamodel** —
  fabricates schema the operator never wrote and drags comments into
  `store.Store`, entitymanager, the audit log, search, `analyze_*`, version
  history and `/_schema`. Far too much blast radius for commentary.
- **Operator-declared comment type** (the `review-response` shape) — free
  search/history/git, but authorship then depends on every operator hand-wiring
  a `bypass_acl` Lua automation; a project that forgets gets client-settable
  authorship, silently.
- **`state.KV` blob storage** — wrong shape for records that must be listed and
  permissioned per target.

**Files to modify:**

- `internal/comments/comments.go` — NEW: `Comment`, `Store` interface, service
- `internal/comments/filecomments/`, `pgcomments/`, `sqlitecomments/`,
  `memcomments/` — NEW backends
- `internal/comments/commentstest/commentstest.go` — NEW conformance suite
- `internal/acl/policy.go` — 4 permission constants + `BuiltinPermissions()`
- `internal/metamodel/types.go` — `Comments *CommentsConfig`
- `internal/metamodel/comments.go` — NEW: config + `CommentPolicy` view type
- `internal/metamodel/loader.go` — `validTopLevelKeys` + `validateComments`
- `internal/appbuild/appbuild_{fs,postgres,sqlite,memory}.go` — backend wiring
- `internal/appbuild/comments.go` — NEW: the permission-guard adapter
- `internal/dataentry/comments_handler.go` — NEW: routes + gates
- `internal/dataentry/api_v1.go` — route registration
- `internal/dataentry/router_walk_test.go`, `readonly_write_route_invariant_test.go`
- `.go-arch-lint.yml` — new components
- `.testcoverage.yml` — if a floor override is needed for the pg-only backend
- `frontend/src/api/comments.ts`, `components/entity/CommentsPanel.vue` — NEW
- `frontend/src/components/common/PropertyDisplay.vue`, `forms/SectionEditForm.vue`
- `docs/metamodel.md`, `docs/data-entry.md`, `docs/data-entry/api-reference.md`,
  `docs/acl-overview.md` + `docs/acl-security.md` (the new permissions)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **`comments:` block (operator config)** — validated at load; `on:` entries
  must name real entity types (allowlist against `m.Entities`); unknown = load error.
- **Comment body (user)** — markdown, rendered only through the SPA's existing
  `renderMarkdown` → DOMPurify path. Never raw.
- **`anchor_ref` (client)** — a property name or `sectionId`; validated as a safe
  identifier. A ref matching nothing is a **warning, not a 422** (DEC-HWZHA).
- **Target entity id/type (client)** — must pass the existing read gate before a
  comment is created or listed.
- **`author` / `created_at` (client-supplied)** — **stripped**. Server writes
  them from `principal.From(ctx)`.

**Security-Sensitive Operations:**

- **Author attribution** — the point of the design. AC3 pins that a
  body-supplied `author` cannot win.
- **Read gating** — a principal who cannot read the target must not learn its
  comments exist (AC5): same indistinguishable-404 as the entity.
- **Commentability allowlist** — a type absent from `on:` is refused (AC4).
- **Principal resolution** — `principal.From(ctx).User` may be a substituted
  entity id (`router.go:465`) or `"unknown"` when unstamped. Store the resolved
  value; refuse rather than persist `"unknown"`.
- **Forward-looking (stage 2, recorded in RES-XRYX18):** if body-level redaction
  ever ships, stored quotes become an unredacted copy of hidden text. Not
  applicable to stage 1 — no quotes are stored.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- **AC1** (absent block = no change) — load a metamodel with no `comments:`;
  assert no `comment` type is synthesised and the new routes 404.
- **AC2** (create + read back) — enable the block, create a property-anchored
  comment, read it via the view path.
- **AC3** (author unforgeable) — POST with `author: "someone-else"` in the body;
  assert the stored author is the request principal. **The key test.**
- **AC4** (type not in `on:`) — refused with a clear error.
- **AC5** (ACL) — following `acl_get_test.go` / `acl_views_test.go`: a principal
  denied the target gets 404, not an empty list.
- **AC6** (detached anchor) — a comment whose `anchor_ref` names a removed
  property still returns, flagged, with no 422.
- **AC5** (independent verbs) — a principal with `comment:read` but not
  `comment:add` lists successfully and gets 403 on POST; one case per verb.
- **AC6** (subject-aware) — a role conferred by an ownership relation to the
  target grants the permission with no global assignment.
- **AC7** (read floor) — comment grants cannot override the target's read
  denial; assert an indistinguishable 404.
- **AC9** (target deleted) — comments for a deleted target are not readable;
  assert the cleanup path.
- **AC10** (conformance) — `commentstest.RunAll` against all four backends;
  postgres gated on `RELA_TEST_DATABASE_URL` like `storetest`.
- **Frontend** — vitest for the affordance in BOTH the `PropertyDisplay` and
  `SectionEditForm` arms; `noNetwork.test.ts` (BUG-762I34) constrains the
  panel's `onMounted` fetch.
- **E2E** — one Playwright spec: select a property, add a comment, see it in the
  panel, resolve it.

**Edge Cases:**

- Empty comment body → refuse (400, malformed wire).
- Unstamped principal (CLI/scheduler) → refuse rather than storing `"unknown"`.
- `principal_property` lookup configured vs not → author is a resolved entity id
  in one case, the raw header in the other. Both stored verbatim.
- Unicode / RTL / very long bodies → stored as markdown, rendered sanitised.
- `anchor_ref` naming an ACL-hidden property → the comment is still gated by the
  target's row verdict; property *names* are not secret (CLAUDE.md).
- Target entity renamed → the relation follows (the store re-keys atomically, #1127).
- Target entity deleted → AC9 (orphaned comment rows need a defined fate).
- Two concurrent POSTs to the same target → both comments survive; covered by
  `commentstest.RunAll` so every backend agrees.
- A comment on an entity whose type is later removed from `on:` → existing
  comments remain readable; new ones refused.
- Many comments on one entity → the view traverse has no cap today; decide
  whether the section needs one.

**Negative Tests:**

- Client-supplied `author` is ignored (AC3).
- Comment on a type absent from `on:` is refused (AC4).
- Comment on an unreadable entity → 404, indistinguishable from missing (AC5).
- `comments:` naming an unknown entity type → **load error**.
- A target id containing path traversal → refused before reaching the file
  backend (`isSafePathSegment` precedent).
- A policy configured but `acl.Request` absent → **fail closed** (deny), per
  `appbuild/transitions.go`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Four backends is the bulk of the work** — and three of them are new storage
  code. *Mitigation:* `commentstest.RunAll` is written first and is the same
  contract for all four; the pg/sqlite ones are a single indexed table.
- **New ACL permissions are a security surface.** *Mitigation:* AC5/AC6/AC7 pin
  independent enforcement, subject-aware conferral, and the read-verdict floor;
  `permguard_test.go` catches an unregistered constant; `/design-review` and
  `rela-security-reviewer` before merge.
- **Postgres-only backend code reads as uncovered in the default CI run** (the
  `internal/store/pgstore` precedent). *Mitigation:* follow the existing
  exclusion/override pattern in `.testcoverage.yml` with a documented reason.
- **Coverage floors** — 50% default, 55 in `dataentry`, 65 in `metamodel`; a
  security boundary invites pressure for more. *Mitigation:* the AC tests are
  most of the coverage.
- **Scope creep into stage 2.** *Mitigation:* store the anchor as
  `{kind, ref}` but accept only `property`/`section` kinds; text-range becomes a
  third kind later without a migration.
- **Effort `l`; the UI is the long pole.** *Mitigation:* the read path needs no
  new component (auto-generated section + existing card renderer), so new UI is
  the affordance + panel only.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- `docs/metamodel.md` — the `comments:` block, beside `attachments:`/`transforms:`.
- `docs/data-entry.md` — the commenting UI.
- `docs/data-entry/api-reference.md` — new routes (required by
  `internal/dataentry/CLAUDE.md`).
- `CLAUDE.md` — that comments are deliberately NOT in the graph, why (the
  `internal/userstate` precedent), and that `author` is server-written.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Run 2026-09-01. 8 findings — 1 critical, 5 significant, 1 minor, 1 nit.

| ID | Severity | Finding |
|---|---|---|
| RR-FCUS1V | critical | Entity rename silently orphans every comment (EntityObserver.EntityRenamed unhandled) |
| RR-60067I | significant | No coherence rule between the 4 permissions; `comment:delete` without `read` loads fine |
| RR-1PCQ42 | significant | Comment count is an unfiltered aggregate / existence oracle |
| RR-7F6NM9 | significant | File-backend concurrency + partial-write durability unspecified |
| RR-OOPBUZ | significant | Comment body unbounded and unvalidated (size, control chars, per-target cap) |
| RR-3VSSPM | significant | Comment ID minting, ordering, and the update/delete subject undefined |
| RR-JT560T | minor | Four backends in one ticket is disproportionate; land file+memory first |
| RR-17JRWP | nit | AC1 "byte-identical" is untestable as written |

**All 8 addressed** — see each RR's `resolution`. Plan changes: a Lifecycle
section (EntityObserver rename/delete), mutating permissions requiring
`comment:read` validated at load, post-gate counts, a stated storage contract
(server-minted IDs, defined ordering, serialised writes, atomic file writes),
allowlist input validation, and two backends instead of four.

**Permission vocabulary settled at 6** (RR-3VSSPM): `comment:read`,
`comment:add`, `comment:update-own`, `comment:update-any`, `comment:delete-own`,
`comment:delete-any`. The graph's ownership mechanism was evaluated and rejected
— it is a graph EDGE test (`graph.HasEdge`, `resolver.go:176`), and comments
have no edge, so reusing it would couple the ACL walker to a non-graph store.
"Own" is a string comparison of stored `Author` against the principal, inside
the comment service.
