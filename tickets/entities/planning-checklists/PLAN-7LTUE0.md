---
id: PLAN-7LTUE0
type: planning-checklist
title: 'Planning: Standalone documents: document: as a navigation entry with optional entity_type'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:

1. `documents:` entries may omit `entity_type` ("standalone documents").
2. New nav entry kind `document: <docName>` in `navigation:`.
3. New route `/document/:name` (no entity id) in the SPA; sidebar href is
computed server-side (see Approach).
4. New API path `GET /api/v1/_documents/{docName}` (no entity segment).
5. Lua document mode: `rela.document.entry_id` is nil for standalone docs.
6. Optional `permission:` on a document — a **global named permission** in the
existing `acl.yaml` `permissions:` family. Absent ⇒ any principal may render
(the Lua read gate still governs content). See Security.
7. Sidebar filtering: a `document:` nav entry whose doc declares a `permission:`
the principal lacks is omitted from `/api/v1/_sidebar`.
8. Docs updates (`docs/data-entry.md`, api-reference, lua-scripting, acl-security).

OUT of scope:

- A new ACL `Op` / `Subject` variant for documents — `permission:` reuses the
existing global-named-permission mechanism instead (see Security).
- Requiring a gate on every standalone doc. Ungated is the default, deliberately.
- `rela.document.depends_on` / SSE dependency tracking (TKT-E1FO1).
- The pre-existing "exactly one nav kind" validation gap (see Research) — a
follow-up ticket.
- Broader TS drift repair beyond `DocumentConfig.script?`, which this ticket
needs.

**Acceptance Criteria:**

1. **Config accepts a standalone document.** A `documents:` entry with `script:`
and no `entity_type` loads without validation errors. *Test:* extend
`TestValidateConfig_Documents` (`validate_test.go:1320`) — cases at 1352-1359
currently *pin* `entity_type is required`, so they must be reworked, not
deleted.

2. **Config accepts `document:` as a nav entry** referencing a standalone doc.
*Test:* table-driven, beside `TestValidateNavigation_KnownAction` (`:1872`).

3. **Nav `document:` referencing an unknown doc is a config error.**
*Test:* mirrors `TestValidateConfig_NavigationUnknownList` (`:1207`).

4. **Nav `document:` referencing an entity-anchored doc is a config error** —
there is no id to route to. *Test:* asserts a distinct, actionable message.

5. **`GET /api/v1/_documents/{docName}` renders a standalone doc**, returning the
same `v1.DocumentResponse` shape. *Test:* `api_v1_test.go` handler test.

6. **Each endpoint shape refuses the other's doc kind** (400 both directions).
*Test:* both asserted.

7. **`rela.document.entry_id` is nil in a standalone render**; `rela.document.id`
is the doc name. *Test:* in `internal/lua/listmode_test.go` beside
`TestListDocumentMode_EntryIDAbsent:103`, reusing its assertion style
(`type(...)`, `~= nil` guard, `or "fallback"`). Keep
`TestDocumentMode_EntryIDStillPresent:121` green.

8. **A doc with `permission:` renders only for a holder.** A non-holder gets 404
(indistinguishable from an unknown doc) and the renderer is never invoked.
*Test:* `acl_documents_test.go`, asserting status + renderer call count == 0.

9. **A doc without `permission:` renders for any principal** — no implicit gate.
*Test:* asserts 200 for a principal holding no permissions.

10. **Sidebar omits a `document:` entry the principal can't render**, and
includes it when they can. *Test:* beside `TestV1SidebarWithNavigation`
(`api_v1_test.go:1967`) + an ACL case in `acl_sidebar_test.go`; asserts `href ==
/document/<name>` in the permitted case.

11. **An unknown `permission:` value is a config error** (typo protection).
*Test:* validation test.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the change is local, and the one open question (how to
gate an entity-less document) resolved to an existing in-repo mechanism.

**Existing Solutions:**

- **Global named permissions are the precedent for a capability with no entity
subject** — `internal/acl/policy.go:36-56`. `PermHistoryRead` ("history:read")
exists *because* a deleted entity has no conferring relations left, so "there is
nothing to evaluate a per-entity verdict against — deleted-history read is
therefore an all-or-nothing global capability, granted via a role's
`permissions:` list". A standalone document is the same shape: no subject
entity, so no per-entity verdict is possible. `RoleDef.Permissions`
(`policy.go:276`) is the grant surface; the `delegate-X` family
(`policy.go:368-408`) is the third instance of the pattern.
- **`WithListDocumentMode` is the direct precedent for a nil `entry_id`** —
`internal/lua/runtime.go:229-249`, a *separate constructor* from
`WithDocumentMode` whose doc comment states `entryID` is "deliberately not a
parameter" because a list render has no entry entity. The conditional it relies
on is `runtime.go:874-876`:
  ```go
  if r.documentEntry != "" { r.L.SetField(docTable, "entry_id", lua.LString(r.documentEntry)) }
  ```
Rationale at `runtime.go:866-873`: `""` is truthy in Lua, so absence beats
empty-string. A standalone doc should get its own constructor for the same
reason, making the nil structural rather than incidental.
- **The kind→href switch is server-side**: `navEntryToSidebarItem`
(`internal/dataentry/views_handler.go:317-356`). `action` leaves `Href` empty
and `Sidebar.vue` renders a button (`Sidebar.vue:176-222`). The SPA consumes the
denormalized `/api/v1/_sidebar` payload, so adding a `document` case is a small
Go change plus a route — not an SPA mapping change. `SidebarItem` has no
entity-id field, which is exactly why standalone docs fit it.
- **Sidebar ACL filtering already exists**: `sidebarCounts`
(`views_handler.go:233-236`) runs through the ACL read scope with a
within-request memo so counts can't alias across principals. Coverage in
`acl_sidebar_test.go:112,132,151,170`.
- `DocumentConfig.EntityType` is already `omitempty` (`config.go:630`); the
requirement lives only in `validateDocuments` (`validate.go:1392-1394`).
- **Command permissions already use this exact field name**: `Permission string`
on a command (`config.go:606`), including a documented case where it is *not*
honored. Reusing `permission:` on a document keeps config vocabulary consistent.

**Pre-existing issues found (noted, not fixed here):**

- `validateNavEntry` (`validate.go:195-226`) has **no "exactly one kind" check**
and no `Label` requiredness — an entry with zero or two kinds validates and
yields an empty `Href`. Adding `document:` widens this by one; follow-up ticket.
- TS `DocumentConfig` (`frontend/src/types/config.ts:362-368`) types `command` as
**required** with **no `script?`** — a script-only doc has no valid TS
representation. This ticket fixes that field (standalone docs are script-first).
- TS `NavigationEntry` (`config.ts:324-336`) lacks `search?`/`settings?`.
- `internal/apiwire/v1/responses.go:239` serializes the Go `DocumentConfig`
directly, which is why the TS drift matters.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Config layer* (`internal/dataentryconfig`)

- `NavigationEntry` (`config.go:433`): add `Document string`, mirroring
`List`/`Kanban`/`Action`.
- `DocumentConfig` (`config.go:623`): add `Permission string`
(`yaml:"permission,omitempty"`), matching the command-permission field name.
- `validateDocuments` (`validate.go:1388`): drop the unconditional
`entity_type is required`. A doc is standalone iff `entity_type` is empty.
`permission:`, when set, must name a permission granted by some role in
`acl.yaml` (typo protection); this needs the policy available to validation — if
it isn't, degrade to a non-empty-string check and note it.
- `validateNavEntry` (`validate.go:195`): add a `nav.Document != ""` branch —
unknown doc ⇒ error; doc with a non-empty `EntityType` ⇒ error.
- Standalone doc with an `edit:` block ⇒ config error (no entity to edit). Must
not depend on distinguishing nil from `{}` (caveat at `config.go:648-651`).

*ACL* (`internal/acl`)

- No new `Op` or `Subject`. Reuse the existing "does this principal hold named
permission P" resolution that `PermHistoryRead` uses. Expose it to dataentry
through whatever accessor already serves the history path; add a narrow
consumer-side interface at the dataentry call site if none exists.

*Sidebar* (`internal/dataentry/views_handler.go`)

- `navEntryToSidebarItem` (`:317-356`): add a `document` case emitting
`Href: "/document/" + name`, icon `document`.
- Filter: when the referenced doc declares a `permission:` the principal lacks,
omit the item — reusing the per-request memo pattern `sidebarCounts` uses so the
check isn't repeated per entry and can't alias across principals.

*API layer* (`internal/dataentry/api_v1.go:2048-2162`)

- `handleV1Documents` accepts one **or** two path segments. Two ⇒ the existing
entity-anchored branch, **unchanged, including the gate-before-type-mismatch
ordering** (`:2093-2103`) whose comment documents the 404-vs-400 oracle. One ⇒
standalone: `isSafePathSegment(docName)` → doc lookup → 400 if it declares an
`entity_type` → permission check (if declared) → render.
- Deny returns the **same 404 body as an unknown doc name**, so doc names aren't
enumerable, and returns *before* the renderer runs.
- Disk cache: `GetCached(ctx, entityID)` (`:2128`) is keyed on entity id;
standalone docs key on doc name so two can't collide. Lua docs bypass the cache
today (`docCfg.Script == ""` guard at `:2127`), so this mainly matters for
standalone `command:` docs.

*Lua layer* (`internal/lua/runtime.go`, `internal/script/executor.go`)

- Add `WithStandaloneDocumentMode(documentID)` beside `WithListDocumentMode`
(`runtime.go:243`), leaving `documentEntry` empty so `runtime.go:874` skips the
field. Add the matching `script.Engine` entry point beside `ExecuteDocument`
(`executor.go:98-114`) to preserve the typed seam.
- **No change to the read gate**: `ReadDeps.VisibleReader` already serves every
read-out and denies when nil (`internal/lua/deps.go:39-48`); dataentry wires the
gated reader at `internal/dataentry/app.go:744`. This is why an ungated
standalone doc is safe by default — see Security.

*SPA* (`frontend/`)

- Route `/document/:name` beside `/document/:name/:entityId`
(`router/index.ts:83-88`).
- `DocumentView.vue`: `entityId` optional; suppress the Edit button (`:65-77`)
when there is no entity.
- `types/config.ts`: add `document?` to `NavigationEntry`; add `script?` and
relax `command` to optional on `DocumentConfig`.

**Files to modify:**

- `internal/dataentryconfig/config.go`, `validate.go` (+ `validate_test.go`)
- `internal/dataentry/views_handler.go` (sidebar href + filter)
- `internal/dataentry/api_v1.go` (routing + permission check), `document.go`
- `internal/lua/runtime.go`, `internal/script/executor.go`
- `frontend/src/router/index.ts`, `views/DocumentView.vue`, `types/config.ts`
- `docs/data-entry.md` (nav table `:1466-1492`), `docs/data-entry/api-reference.md`,
`docs/lua-scripting.md`, `docs/acl-security.md`
- Tests per the AC list

**Alternatives considered:**

- *Require `read_subject: <entity_type>` on every standalone doc.* Rejected —
borrowing an entity type as a proxy for "may see this report" is indirect and
conflates two things; and requiring it on every doc is unjustified given the Lua
read gate already bounds content (see Security). A named permission says what it
means.
- *New `OpViewDocument` + `DocumentSubject`.* Rejected — `Subject` is a sealed
switch; a doc has no entity to be a subject. `PermHistoryRead` exists precisely
for this shape.
- *Synthetic "singleton" entity.* Rejected — the arbitrary-anchor workaround the
ticket exists to remove.
- *Sentinel id `/document/:name/_`.* Rejected — a sentinel that must never
collide with a real id is a latent bug; `entry_id` would be non-nil for nothing.
- *Pass `""` to `WithDocumentMode`.* Rejected — exactly the empty-string footgun
`runtime.go:866-873` warns about.
- *`dashboard:`-style boolean.* Rejected — documents are named.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Why ungated-by-default is sound here.** Entity-anchored documents are
ACL-gated through the entry entity (`api_v1.go:2099`), and that gate's comment
(`:2093-2098`) cites Lua reading related entities. Dropping `entity_type`
removes that subject — but it does **not** remove the read gate, because
document Lua reads go through `ReadDeps.VisibleReader`, which "serves every
READ-OUT" and *denies* rather than falling back when nil
(`internal/lua/deps.go:39-48`, RR-X9NVHI). The data-entry runtime wires the
gated reader (`internal/dataentry/app.go:744`).

Consequence: a principal who may not read the underlying entities renders an
**empty or partial report**, not a leak. The content gate is per-entity and
already correct. So a document-level gate is a **UX and intent affordance** —
"don't show me reports I can't use", and "this report is for the directie" — not
the confidentiality boundary. Requiring one on every doc would be ceremony.

Two caveats that keep this honest:

- **Aggregates over permitted data are still derived data.** If a principal may
read the entities individually, a report that aggregates them reveals nothing
new. `permission:` exists for the case where the *composition* is sensitive even
though the parts are readable.
- **The gate must precede the renderer.** Even though content is safe, running
an expensive Lua aggregation for a denied caller is a DoS and side-effect
surface. The permission check returns before `Render` is invoked, mirroring the
existing ordering comment.

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
|---|---|---|---|
| `docName` path segment | HTTP client | `isSafePathSegment` (`api_v1.go:2069`) — flows into the cache filename | 400 `invalid_path` |
| `document:` nav ref | config author | allowlist against `cfg.Documents` at load | config error |
| `permission:` | config author | must be a permission some role grants | config error |
| entity id (2-segment form) | HTTP client | unchanged | unchanged |

**Security-Sensitive Operations:**

- **Lua / `command:` execution** — for a gated doc, reached only after the
permission check; `command:` confinement via `internal/cmdexec` unchanged.
- **Disk cache write** — key moves from entity id to doc name; `docName` is
`isSafePathSegment`-validated before any filesystem work.
- **Deny response** — byte-identical to the unknown-doc 404. No distinguishing
message, so document names stay non-enumerable.
- **Sidebar filtering** — a UX affordance, *not* the boundary; the endpoint
re-checks. Mirrors the `_actions` rule in `internal/dataentry/CLAUDE.md` ("Don't
trust `_actions` for authorization").
- **Edit button** — suppressed for standalone docs rather than emitting a broken
link.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** each AC names its test and the existing test it extends.

- `internal/dataentryconfig/validate_test.go` — AC1-4, 11. **`TestValidateConfig_Documents`
cases 1352-1359 pin `entity_type is required`; rework, don't delete.** The
comment at 1316-1319 ties them to RR-1FA8W (relaxing `command` requiredness must
not silently drop a sibling requirement) — keep an equivalent case.
- `internal/dataentry/api_v1_test.go` — AC5, 6, 10 (href shape).
- `internal/dataentry/acl_documents_test.go` — AC8, 9. AC8 asserts the renderer
is **not invoked** on deny, not merely the status.
- `internal/dataentry/acl_sidebar_test.go` — AC10 filtering, both directions.
- `internal/lua/listmode_test.go` — AC7.
- `frontend` — AC10 route test. (`Sidebar.vue` has no unit test today; acceptable
since the href and filtering are server-computed and tested in Go.)

**Edge Cases:**

- Standalone doc, no `permission:` → renders for everyone (AC9).
- `permission:` naming a permission no role grants → config error.
- Doc with both `entity_type` and `permission:` → decide: likely allow
(belt-and-braces, both checks fire) but document the order; do **not** let the
permission *widen* the entity gate.
- Nav `document:` → entity-anchored doc → config error.
- `GET /_documents/{name}` on an entity-anchored doc → 400; the converse → 400.
- `docName` with `..`, `/`, null bytes → 400 (existing guard).
- Two standalone `command:` docs → distinct cache keys.
- Standalone doc whose script errors → existing structured Lua error path.
- Standalone doc with `edit:` → config error; bare `edit:` deserialises to nil
(`config.go:648-651`), so the check must not rely on nil-vs-`{}`.
- Denied principal whose sidebar omits the entry still gets 404 on a direct URL.
- Hot-reload of `data-entry.yaml` re-runs these validations (TKT-IMBOK).

**Negative Tests:**

- Non-holder on a gated doc: 404, body byte-identical to the unknown-doc 404,
renderer call count == 0.
- Unknown doc name → 404. Non-GET → 405.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| Reworking the `entity_type is required` cases silently drops the RR-1FA8W coverage they encode | Medium | Rework, don't delete; keep an equivalent invariant case |
| Relaxing `entity_type` weakens the entity-anchored type-mismatch check | Medium | Separate branches; each endpoint rejects the other's kind; gate ordering preserved verbatim |
| Permission validation needs `acl.yaml` at config-validate time; it may not be plumbed there | Medium | Check during implementation; degrade to a non-empty check + a startup warning rather than skipping validation silently |
| Sidebar filter treated as the boundary | Medium | Endpoint re-checks; test asserts direct-URL 404 for a principal whose sidebar omits the entry |
| Disk cache key collision between standalone `command:` docs | Low-Med | Key on doc name; test |
| Widening the existing "no exactly-one-kind nav validation" hole | Low | Out of scope; follow-up ticket |
| `DocumentView.vue` assumes `entityId` | Low | Optional prop; suppress Edit button |

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — standalone documents; `document:` in the navigation
table (`:1466-1492`); `permission:` semantics and the ungated default
- [x] `docs/data-entry/api-reference.md` — `GET /_documents/{docName}`
- [x] `docs/lua-scripting.md` — `entry_id` nil in standalone mode
- [x] `docs/acl-security.md` — document `permission:` beside the existing named
permissions (`history:read`, delegate-X)
- [x] `internal/dataentry/CLAUDE.md` — one line: a standalone document's
`permission:` is an intent/UX gate; the Lua read gate remains the
confidentiality boundary
- [ ] `docs/metamodel.md` — N/A (this is `data-entry.yaml`, not the metamodel)

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- pending -->
