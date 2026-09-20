---
id: TKT-Z8K2FS
type: ticket
title: 'Duplicate an entity from the detail page: relation-picker modal, then a prefilled create form'
kind: enhancement
priority: medium
effort: l
status: done
---

## Description

Add a **Duplicate** action to the entity detail page (`/entity/:type/:id`). It
opens a modal listing the entity's relation types in both directions, each with
a checkbox and an edge count. On confirm the user lands in a create form
prefilled from the source's properties and content, with the chosen relations
pre-linked. On submit the new entity is created and opened.

Nothing is written until the user submits. That is the load-bearing choice — see
"Why not a direct server-side copy".

## What already exists

| Piece | Where | State |
| --- | --- | --- |
| Orphaned clone endpoint | `internal/dataentry/write_handler.go:1183` `handleV1CloneEntity`, routed `api_v1.go:1431` | Ships. Copies properties + content, mints a fresh id, lands on the source's face, read-gates the source. **Copies zero relations. Zero callers — no frontend code, no e2e test.** |
| Both-direction relation listing | `GET /{plural}/{id}/relations`, `api_v1.go:1122` | Works, ACL-gated per peer (`relation_visibility.go:69`). **No frontend client function for the all-relations shape** (`api/entities.ts:300` is per-type). |
| Property prefill by query param | `DynamicForm.vue:684-698` `?prop.*` / `?rel.*` | Works on a create **page**. Strings only. |
| Batched create | `POST /{plural}` with `relations`, `write_handler.go:365`, `:445` | One call creates the entity and all `link_as: to` edges. |
| Per-relation create gate | `gateCreateRelationAffordances`, `write_handler.go:300` | Runs before `CreateEntity`; a `relations:` body cannot bypass a `creatable: false` verdict. |
| Affordance-object precedent | `_copies`, `affordances.go:1384-1437` | The shape to copy: omitted, never empty, when unwired. |
| Detail-page action invocation | `runCopy()` `EntityDetail.vue:815-856`, `landAfterCopy()` `:874-905` | The pattern for "capture the subject before the await, then navigate". |

## Why not a fifth `_actions` verb

`_actions` is a closed set of four verbs (`create`/`update`/`delete`/`rename`,
`affordances.go:132-135`). `translateVerb` **panics** on an unknown verb
(`:67`), and `lint_test.go:29` confines `acl.WriteRequest{Op:` construction to
that one file. Two prior tickets (TKT-R4BMJM, TKT-WRLDAPI) already rejected
extending it.

Duplicate needs no new verb anyway: it is `create` on the collection plus `read`
on the source, both already computable. The affordance rides the entity response
as a sibling key, like `_copies`.

## Why not a direct server-side copy

Copying properties verbatim breaks on any type with a `unique:` property.
`checkUniqueProperties` (`internal/entitymanager/unique.go:56`) runs on the
create path and raises a hard 422 whose message **deliberately withholds the
colliding entity and value** (`:106-123`, an enumeration oracle). The user would
get an error and no copy, with nothing actionable in the message. The orphaned
clone endpoint has exactly this defect today.

Routing through the create form makes the collision editable before it is a
failure, and inherits validation, templates and the dry-run preview for free.

Two limits on that guarantee, worth knowing since the whole design rests on it.
`unique:` is skipped entirely for **list** properties (`unique.go:69-71`), and
the loader rejects `unique:` only on a non-string type (`loader.go:1157-1162`) —
so `unique: true, list: true` loads clean and enforces nothing. Enforcement is
also **per-face** (`unique.go:82-102`) and check-then-write on fs/mem. A
`unique:` list property therefore needs no acceptance criterion: it cannot
collide.

## What the copy carries

**All properties the caller can see, plus the markdown content, by default.** An
operator may narrow that with an explicit list per type:

```yaml
entity_views:
  invoice:
    duplicate:
      properties: [customer, currency, line_items]   # optional; default = all
```

**The config home is `entity_views:`, not `views:`.** `views:` is keyed by view
**id**, not entity type (`internal/dataentryconfig/config.go:103`), so a type
may have several views or none and `views.invoice` means "a view named invoice".
`entity_views:` (`config.go:104`) is the per-entity-type map, its docstring is
"UX bindings for a metamodel entity type", it is already validated per type
(`validate.go:935-963`), and it already ships to the SPA on `/_config`
(`api_v1.go:1763`) so the SPA needs no new plumbing. It also costs no plimsoll
budget: `Config` is pinned `//plimsoll:max-fields=22` at exactly 22 fields, so a
new **top-level** key would need a deliberate pin bump, while nesting does not.

Absent a `duplicate:` block, every visible property and the content carry over.
The id is never carried (a fresh one is minted); the face always is (a duplicate
of a draft is a draft).

### The prefill marks its keys as touched (RR-DDY9LG)

`visibleWritablePropertiesForCommit` (`DynamicForm.vue:491-512`) keeps a key
unconditionally only when it is in `userTouched`; otherwise it omits any key the
create dry-run reports hidden or read-only. `applyTemplate` writes `formData`
directly and never touches `userTouched`, so a prefilled property the user never
types into would be **silently dropped from the commit** — the user sees it in
the form, submits, and the copy lacks it, with no error.

The duplicate prefill therefore registers its keys as `userTouched`. The
existing rationale at `:497-502` (RR-2U2D) says why that is safe rather than a
bypass: the server's affordance gate (BUG-Q60V) rejects denied writes, so an
under-resolved touched key that policy actually denies still 403s at commit with
a clear `rule_id`. Failing loudly at the boundary beats dropping a value the
user can see.

### `file` properties are never carried

Attachments live at `attachments/<entityID>/<property>/<fileName>`
(`internal/attachment/attachment.go:95`) and the property value is only the
filename. Copying a `file` property would point the duplicate at bytes stored
under the SOURCE's id, so the copy renders an attachment that 404s on download.

`file`-typed properties are therefore **excluded from the prefill**.
"Attachments out of scope" must mean excluded, not carried-and-broken.

### State-machine properties are never carried

A state-machine property must NOT come from the source. `applyCreateLock`
(`internal/dataentry/affordances.go:1136-1190`) states the rule: "A create is an
ENTRY, not a transition" — on create the machine field is pinned to its entry
value, marked read-only, and dropped from `_transitions`, because offering moves
on an entity that does not exist is nonsensical.

So duplicating a ticket with `status: done` must yield a ticket at the entry
status, not `done`. Carrying the source value would be rejected by the server on
submit anyway, so the failure mode is a confusing 422 on most types in this repo
rather than a data leak — but it must be handled at prefill time, not discovered
at submit.

The duplicate prefill therefore excludes machine-typed properties and lets the
existing create path supply the entry value. A machine field hidden by
field-visibility policy stays stripped (RR-C3OJ33) and is not re-added.

### Redacted properties: why "all by default" is legal here but not for `copies:`

The metamodel **refuses** `fields: all` on a cross-entity `copies:` definition
at load time (`internal/metamodel/copies.go:120-132`): such a copy reads through
the caller's visibility gate, so copying "everything" writes whatever survived
redaction as though it were the whole entity, destroying fields the principal
could not see. That is the redacted read-modify-write the codebase forbids
everywhere.

This ticket's default sounds identical and is not, because the operation is
different. A `copies:` invocation is a **server-side write** of a full entity. A
duplicate is a **create**: the new entity is built from what the user was shown
and submitted, so there is no pre-existing target whose hidden fields could be
clobbered. Nothing is destroyed, because nothing is overwritten.

What remains true is that a duplicate of an entity with redacted properties is
**incomplete**, and the user must be told. The client already has the signal:
`_redacted` (`frontend/src/types/entity.ts:47`) names the withheld properties
explicitly, because inferring hiding from absence conflates "redacted" with
"never set" (BUG-MLT9DE). The modal reads `_redacted` and names the properties
it could not carry. Silently producing a short copy is the one unacceptable
outcome.

## The modal

One row per relation type that has at least one visible edge, grouped by
direction (Outgoing / Incoming), each row showing the type label and edge count,
with a checkbox. Checking a type carries **all** its visible edges. Per-edge
selection is out of scope — see below.

The list is built from `GET /{plural}/{id}/relations`, which already drops edges
whose peer the caller may not read, so the user can only see, and therefore only
copy, relations they are entitled to.

Default check state: all outgoing types checked, all incoming types unchecked.
An incoming edge means "something else points at me", which is usually not part
of what the user is duplicating; the checkbox is there when it is.

## Enablement

Available by default on every entity type the principal may create — no config
needed to get the action. This does **not** reverse TKT-651W's read-only-view
invariant: duplicate opens a create form, it does not edit the graph from the
read view. The `duplicate:` block only narrows what carries over; it is not an
opt-in switch.

## Flow

1. Detail page header renders **Duplicate** when the entity's type appears in the
sidebar `inline_create` map, reusing the form id it carries.
2. Click opens the modal. It fetches the relation summary and renders the checkbox
list.
3. Confirm swaps the modal to an embedded `DynamicForm` for the type, passed the
source's properties, content, selected relation peers and world as props.
4. The user edits whatever needs editing — in particular any `unique:` property
that would otherwise collide — and submits.
5. Submit uses the existing batched create, which writes BOTH directions in one
call: `resolveDirection` (`relations_direction.go:41-57`) maps an inverse body
key back to its canonical relation and flags it incoming, and the apply path
(`relations_modern.go:274`) honours that. The client passes relation keys
through exactly as the read endpoint returned them — no key translation, no N+1,
no post-create partial-failure window for edges.
6. The user lands on the new entity's detail page.

**Prefill transport: props to a modal-embedded form. Settled, not open.**

The flow already opens a modal to pick relation types, so the create form is
hosted in that same modal and the payload is passed as props. There is no
navigation to carry a payload across, so the transport problem dissolves rather
than being solved. `EntityDetail.vue:809-828` already documents this as the
sanctioned rule: "PAGE navigates with query params... MODAL passes props. An
embedded form reads an EMPTY query on purpose."

The URL channel was considered and **rejected on evidence**:

- It corrupts data. `DynamicForm.vue:680` skips any non-string value, so a
duplicated `list:` property is silently dropped entirely, and `:768-770` assigns
raw strings with no coercion, so numbers, booleans and dates all round-trip as
text. `applyTemplate` (`:936-938`) assigns typed JSON instead.
- There is no content channel at all. `initializeDefaults` never touches
`content`; in create mode the markdown body has exactly two sources, the
template and the user typing.
- It exceeds realistic limits on **typical** data, not outliers. The dogfood
ticket corpus averages ~3 KB of markdown (50 KB at the tail), which is ~6-9 KB
percent-encoded before properties or relation params. No proxy budget is
documented anywhere in the repo, and Go's 1 MB default header limit means this
would pass in dev and 414 behind a production reverse proxy.

`history.state` and `sessionStorage` were also rejected: neither has any
precedent in this codebase (the only `sessionStorage` uses are chunk-reload
throttles), `history.state` breaks on a new tab or pasted URL, and stashing a
duplicate payload in web storage would write possibly-redacted property values
to browser disk.

The apply path already exists. `applyTemplate` (`DynamicForm.vue:935-955`)
writes typed properties, content and relations, then re-baselines `originalData`
so the unsaved-changes guard does not immediately fire. **A duplicate candidate
is a template computed from an entity**, so it reuses the `Template` type
(`frontend/src/types/schema.ts:174-184`) rather than inventing a parallel one.

Consequences accepted with this choice:

- **No deep link.** A duplicate cannot be shared as a URL and a mid-flow reload
loses the draft. Correct for an action taken *from* an entity you are looking
at, but it differs from every other create surface, which are all linkable.
- **Embedded-form constraints are inherited.** An embedded form takes its world
from `props.embeddedWorld`, not the route (`DynamicForm.vue:1522-1528`), so the
source's world must be threaded explicitly or a faced type lands on the wrong
face.
- **If a page flow is ever needed**, the fallback is NOT a bigger URL. It is a
server-computed candidate endpoint returning a `v1.Template`
(`internal/apiwire/v1/responses.go:882-887`), with the URL carrying only ids.
That must answer the objection in `frontend/src/api/copies.ts:5-14`, which
argues against adding an endpoint for a single affordance.

## Scope

**In scope**

- Duplicate button on the entity detail page, desktop **and** the mobile overflow menu.
- Relation-picker modal: both directions, grouped by type, counts, checkboxes.
- Prefill of properties and content from the source into the create form.
- Pre-linking the selected relation types' edges, both `link_as: to` and `link_as: from`.
- Optional `duplicate.properties` narrowing per type, with load-time validation.
- A frontend client function for the all-relations endpoint (none exists).
- Face preservation: a duplicate of a faced entity lands on the source's face.
- A `duplicate:` block under `entity_views:` with load-time validation, plus
relaxing `validateEntityViews` so a `duplicate:`-only entry loads.
- SPA plumbing for `entity_views` (type, store field, reader) — none exists.

**Out of scope**

- Per-individual-edge selection. Per-type is the unit; a user wanting a subset of
one type's peers can unlink after.
- Deep/recursive duplication (copying the peers themselves, not just the edges).
- Duplicating relation edge **properties** onto the new edges. `RelationCards`
territory, and the batched create body has no vocabulary for it.
- Duplicating attachments or comments.
- Bulk duplicate from a list view.
- Extending the `_actions` verb set.

## Acceptance criteria

1. The Duplicate button appears on the detail page when the entity's type is present
in the sidebar payload's `inline_create` map, and is absent otherwise. **Not**
`_actions.create`: `create` is a COLLECTION-scope verb (`affordances.go:135`),
so an entity response carries only `update`/`delete`/`rename` and has no
`create` key at all. `inline_create` (`responses.go:803-821`) encodes exactly
the two conditions needed — the principal may create the type AND a create form
resolves for it — and ships the resolved form id, so the client does no
permission arithmetic and no form lookup. It is a UI hint, never authorization;
the create re-authorizes. It is present in the mobile overflow menu as well as
the desktop header — `hasOverflow` (`EntityDetail.vue:1042`) must account for
it; three prior features shipped desktop-only and regressed here.
2. The modal lists every relation type with at least one visible edge, in both
directions, with correct counts. A type whose every edge has a hidden peer does
not appear at all.
3. Outgoing types default to checked, incoming to unchecked.
4. An entity with no visible relations opens the modal with an **explicit empty
state** — it does not skip the modal. Skipping is wrong once the fail-closed
fetch (AC17) is considered: a transient store error would silently become a
zero-relation duplicate with no confirmation step at all.
5. Confirm opens a create form whose property fields and content are prefilled from the
source, and whose relation fields contain the selected types' peers.
6. Submitting creates the entity with all selected edges present, in both directions,
verified by reading the new entity's relations.
7. Unchecking a type means none of its edges exist on the copy.
8. A type with a `unique:` title duplicates without a 422 **because the user edited the
title in the form first**; submitting an unedited colliding title produces the
normal 422 surfaced as a field error, not a silent failure.
9. `entity_views.<type>.duplicate.properties: [a, b]` carries only `a` and `b`;
properties outside the list take their metamodel/template defaults, they are not
blanked.
10. Config load rejects an invalid `duplicate:` block: a property name the type does not
declare, a non-mapping value, or an empty `properties:` list (absence is how a
type opts out of narrowing). Model the validator on `validate.go:2311-2317`,
which already reports an unknown property name for a type. 10a. Duplicating an
entity with redacted properties names the properties that could not be carried,
read from `_redacted` — which covers **field-level `visible:` redaction only**,
not git-crypt `inaccessible` content or relation-meta redaction (a different
field on a different wire type). A short copy is never produced silently. 10b. A
state-machine property is NOT carried from the source: duplicating an entity at
a non-entry state yields the entry state, per `applyCreateLock`
(`affordances.go:1136-1190`). Asserted on a type whose machine has a non-entry
current value. 10c. The duplicate modal calls `provideInlineCreateDepth()`, so
its embedded form is at depth 1 and its relation pickers offer link-existing but
not inline-create (`useInlineCreate.ts:23`, MAX_INLINE_CREATE_DEPTH = 0).
Modal-in-modal is structurally unreachable because `modalStack` is a Set and
cannot say which dialog is topmost; the duplicate must not be the first surface
to violate that.
11. A duplicate of a faced entity lands on the source's face. For a faced type the face
is never omitted — omission is a refusal, not a default.
12. A forged request carrying a relation type the principal may not create is refused
by `gateCreateRelationAffordances`, asserted on **both** directions against a
schema declaring a non-creatable verdict — undeclared relation types are
default-permissive, so a verdict-free schema passes vacuously. Note both
directions traverse **one gate with two inputs**, not two code paths: incoming
edges ride the same create body under inverse-named keys and
`validateRelationsModernAffordances` (`affordances.go:646-709`) resolves an
incoming verdict against the peer as source.
13. A forged duplicate naming a source the principal may not read is refused
indistinguishably from a 404.
14. The detail-page query count is unchanged by the affordance at 10 and 50 relation
rows (`querybudget_test.go` budget test). The relation summary is fetched when
the modal opens, not on page load.
15. Non-string property types survive the duplicate intact: a `list:` property, a
number, a boolean and a date all carry their typed values onto the copy. This is
the specific failure mode that ruled out the URL transport, so it is asserted
directly rather than assumed from the choice.
16. The markdown content carries over. Create mode has no content prefill channel
today, so this is new behaviour and needs its own assertion.
17. A failed relation fetch is surfaced as an error, never as an empty list with a
live Confirm. `visibleRelationIDs` (`relation_visibility.go:82-86`) drops every
neighbour fail-closed on a store error and the endpoint still answers 200 with
an empty map, so "transient error" and "genuinely no relations" are
indistinguishable on the wire. Failing closed is correct for the gate; silently
duplicating with no relations because of it is not. The client must distinguish
a failed fetch from an empty result.
18. Duplicating a self-referencing entity with both a relation and its inverse checked
does not trip `detectSelfLoopShapeConflict` (`relations_direction.go:89`) with
an opaque error; either the payload is built so the conflict cannot arise, or
the conflict is reported in terms the user can act on.
19. Prefilled property values reach the created entity — including values on fields
the user never edits. Asserted directly, because the commit filter drops
untouched keys the dry-run reports hidden or read-only (RR-DDY9LG). 19a.
Prefilled relation edges survive `pruneWizardHiddenRelations` and the
`cardRelations` exclusion. RR-7Z3SFC fixed this for a SINGLE pre-linked peer
with a post-condition checking only `linkParams.as === 'to'`
(`DynamicForm.vue:1451-1481`); a duplicate prefills many peers across many
types, so the same silent-drop reappears at N× the surface unless the
post-condition generalizes (RR-VR2YGE). 19b. A `file`-typed property is not
carried onto the duplicate, so the copy has no dangling attachment reference.
19c. An `entity_views` entry carrying only a `duplicate:` block loads without
error. `validateEntityViews` (`validate.go:943-947`) currently rejects an empty
`detail_view`, so it must be relaxed to "at least one of `detail_view` /
`duplicate`" (RR-LDPV50). 19d. `GET /{plural}/{id}/relations` has a query budget
test: it is the endpoint that scales with relation count, and it is unbudgeted
today.
20. The duplicate modal registers with the modal stack (`composables/modalStack.ts`),
so the detail page's Del/Backspace delete shortcut (`EntityDetail.vue:619-625`)
cannot fire while it is open.
21. An e2e test drives the full flow: open detail, click Duplicate, uncheck one type,
confirm, edit the title, submit, and assert the new entity exists with exactly
the expected edges. It must assert on real markup —
`e2e/pages/form.page.ts:415-417` records that the previous inline-create test
was vacuous against loose selectors.

## The orphaned clone endpoint: superseded, removed separately

`POST /{plural}/{id}/_actions/clone` (`write_handler.go:1183`, routed
`api_v1.go:1424-1436`) is **not** this feature's backend, and the duplicate does
not extend it.

It arrived with the original Vue SPA migration (#230) as speculative v1 surface;
that commit message never mentions clone, and no ticket ever asked for it. It
has zero callers — no frontend code, no Go test, no e2e spec, no Lua script, no
MCP tool. Three later tickets have nonetheless paid maintenance on it (TKT-VQGN
added the read gate, TKT-HKY8RJ moved it onto `writeHandler`, BUG-HC6I2T fixed
its dropped face), each by sweeping all entity-addressing endpoints rather than
by intent.

It is also wrong in a way this feature cannot inherit: it copies properties
verbatim, so it 422s on any type with a `unique:` property, and it copies no
relations. Extending it would also contradict the design: a server-side clone is
write-first, and this feature's load-bearing choice is that nothing is written
until the user submits.

Removing it is cheap and unblocked. The `/api/v1` surface is **not** declared
stable anywhere; the only written policy (`internal/dataentry/CLAUDE.md:48-50`)
governs `_actions` **verbs** — the affordance map — and `_actions/{action}` is a
different concept that merely shares the name. No lint invariant covers the
route: `lint_test.go` guards `acl.WriteRequest{Op:` construction, not URL paths.
Clone is the only `case` in `handleV1EntityAction`, and no test asserts on it.

**It is removed in a separate ticket, not this one.** Deleting a published
endpoint (it is emitted per entity type into `/api/v1/_openapi.json` via
`internal/openapi/paths.go:301-322`) is an API change with its own blast radius
— handler, route, the now-empty `handleV1EntityAction`, the `_actions` path arm,
the OpenAPI emitter, and a `.golangci.yml:349` exemption. Bundling that into a
feature ticket mixes an externally-visible removal into a UI change and muddies
the revert. This ticket's obligation is only that it must not build on clone and
must not leave two copy paths presented to users; a follow-up ticket does the
removal.

## Risks

- **Affordance/write divergence.** A Duplicate button offered for something the write
path refuses. Mitigated by deriving from `inline_create`, which the server
computes from the same create verdict the write path enforces, never from "the
detail page rendered".
- **Self-loop inverse conflict.** `detectSelfLoopShapeConflict`
(`relations_direction.go:89`) rejects a body naming both a relation and its
inverse for the same self-loop, reachable when duplicating a self-referencing
entity with both directions checked. Build the payload so the conflict cannot
arise, or report it in terms the user can act on.
- **Counts are per-principal by construction.** The all-relations endpoint returns
no counts and no envelope; the client derives them with `.length` per group. The
response is unpaginated, so a derived count is a true total — but a total over
*visible* peers only. That is what makes "a type whose every edge has a hidden
peer does not appear" work. Do not later "fix" the count against a raw graph
total: that reopens an existence oracle.
- **Relation enumeration cost.** The all-relations endpoint is a separate fetch;
issuing it on detail-page load rather than modal-open would tax every page view
for a feature most views never use.
- **Peer visibility recheck.** The modal's list is read-gated, but a peer may become
hidden between modal-open and submit. The create path's own gate is the
backstop; the failure must surface, not silently drop an edge.

## Implementation constraints

- **`App` has zero plimsoll headroom.** `internal/dataentry/app.go:205-206` pins
`max-methods=90` / `max-exported-methods=23` and `App` sits at exactly both. Any
new `App` method fails CI. The comment there records a previous ticket hitting
this wall and the sanctioned fix: a package function taking its seams
explicitly. Put compute logic on `affordanceService` (which carries no directive
and already hosts `computeCopyOffers`), on a small handler struct like
`copiesHandler`, or in a package function — never on `App`.
- **No new affordance key is needed.** Enablement reuses `inline_create`
(RR-ULJ0WK), which already encodes "may create AND a form resolves" and ships
the form id. That removes the `_duplicate` wire key, the "omitted never empty"
discipline and the `_copies` pattern-matching from this ticket's scope.
- **`entity_views` reaches the wire free, but not the SPA.**
`internal/apiwire/v1/responses.go:600` reuses
`dataentryconfig.EntityViewConfig`, so a new Go field is on the wire with no
plumbing. The SPA side is NOT free: a repo-wide grep finds zero references to
`entity_views` in `frontend/src`, and `detail_view` has no reader outside
config, validation and tests. The SPA needs the type, the store field and the
reader (RR-LDPV50).
- **`docs/data-entry.md` is generated** from
`docs-project/entities/guides/GUIDE-data-entry.md` via `just docs`. Edit the
guide entity, never the generated file (RR-WGNYNY). The generator rewraps prose,
so run `just docs` twice and confirm no diff — `just docs-check` runs only in
`just ci`, not in `just lint` or `just test`.
- **Naming.** The repo will briefly hold three words for adjacent operations:
`clone` (being removed), `copies:`/`_copies` (schema-level face copies), and
`duplicate` (this). Keep `duplicate` clearly distinct from `copies:` in docs.
Three overlapping names is how the `_actions` collision happened.

## Considered and rejected

- **Extending `copies:` in `schema.yaml` with a `new <type>` self-copy.**
Architecturally tempting: the field map, its property validator, per-relation
copy verbs and the `_copies` wire affordance all already exist and are tested.
Rejected because `copies:` is invoke-by-name and **write-first**, which is the
opposite of this feature's load-bearing "nothing is written until the user
submits", and because a cross-entity copy inherits the `fields: all` load
refusal above. Recorded because "why isn't duplicate just a copy definition?" is
a fair question with a real answer.
- **A standalone duplicate-prefill endpoint.** `frontend/src/api/copies.ts:5-14`
records that an earlier `GET /_copies` endpoint was removed in favour of riding
the entity response. A modal-embedded form needs no endpoint at all, since it
already has the entity. If a page flow is ever added, the endpoint returns a
`v1.Template` and must answer that objection.
