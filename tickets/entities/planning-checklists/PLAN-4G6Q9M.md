---
id: PLAN-4G6Q9M
type: planning-checklist
title: 'Planning: Create related entities from the entity detail page (section + header buttons, modal or page flow)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** the read-only entity detail page (`/entity/:type/:id`) lists
related items but offers no way to create one. The user must navigate away, find
the right create form, fill it in, then link the relation by hand — and the link
step is easy to forget, producing orphans.

**Scope:**

IN:

- A `create:` block on `ViewSection` (`internal/dataentryconfig`), validated at config load.
- Add-target resolution extended from the `_sidepanel`-only path to the `_views` detail path.
- Threading the originating relation from the `traverse:` rule onto the section affordance.
- Affordance fields on the `_views` wire response (`v1.ViewSection`).
- Section-header buttons and an aggregated page-header menu in `EntityDetail.vue`.
- Two flows: modal (embedded form, refresh in place) and page (navigate, return to origin).
- Template variant preselection, keyed per target type.
- Fixing the `_relation`/`_linkAs`/`_peerId` vs `link_relation`/`link_peer`/`link_as`
param mismatch in `SidePanel.vue` (the existing Add button never pre-links).

OUT:

- Linking *existing* entities from the detail page (the `LinkInfo` equivalent).
- Reworking `EntityDetail.vue`'s six-branch section render (FEAT-KQ45P owns that).
- Relation edge properties on the created link (`RelationCards` territory).
- Extending the `_actions` closed verb set — it has no vocabulary for "create type X
via relation Y" and is guarded by a lint invariant. The affordance rides the
section.
- Server-side template application. `CreateOptions.Variant` exists
(`internal/entity/writeapi.go:22`) but only `webhook_routes.go:490` uses it, and
the SPA create handler (`write_handler.go:281`) neither accepts a template key
nor sets `Variant` in `parseCreateOpts` (`:258-280`). Its decode is **strict**
(BUG-HC6I2T), so a client sending `template:` gets a 422 rather than a silent
ignore. Adding the server channel means extending both the request struct and
`parseCreateOpts` — out of scope here; the SPA applies templates client-side and
this ticket stays consistent with that.

**Acceptance Criteria:** the ten criteria on TKT-R4BMJM, each mapped to a test
in the Test Plan below.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — no external survey needed. The approach is determined by
in-repo prior art (TKT-OMUD56, settled by design review) rather than by a choice
between competing external options. Every mechanism already exists in the
codebase; the work is routing them to a surface that lacks them.

**Existing Solutions:**

No third-party library applies — this is a config-schema plus UI-affordance
change against rela's own view pipeline.

Prior art found in the codebase, all reusable:

| Mechanism | Location | Reuse |
| --- | --- | --- |
| Add-target derivation | `internal/dataentry/sections.go:436` `resolveSectionButtonsWithTraverse` | Generalize from `_sidepanel` to `_views` — **and add the missing ACL gate** (see below) |
| Create-permission check | `internal/dataentry/affordances.go:154` `computeCollectionActions` | **Not currently called by the resolver at all**; must be wired in |
| Form resolution per type | `internal/dataentry/views_handler.go:773` `createFormForType` | Unchanged |
| Pre-link query params | `DynamicForm.vue:673-678`; reverse link `:1414-1434` | Page flow uses as-is |
| Return-to-origin | `utils/returnPath.ts`, `useBackTarget.ts`, `DynamicForm.vue:1509` | Page flow uses as-is; open-redirect guard already present |
| Template variants | `templating.EntityTemplates`, `GET /api/v1/_templates/{type}`, picker `DynamicForm.vue:745-751` | Add a preselect channel |
| Modal create host | `InlineCreateFormModal.vue`, `useInlineCreate.ts` | Reusable outside a form (see below) |
| Per-type keyed config | `ViewSection.ParentColumns` / `ChildColumns` | Precedent for the `types:` map shape |
| Default view synthesis | `internal/dataentry/default_view.go:33` `buildDefaultViewConfig` | Gives most types a usable section shape for free |

Three findings that shape the design:

1. **`InlineCreateFormModal.vue` is reusable from `EntityDetail.vue`.** It depends only
on `useModalStack`, `useConfirm` and `useSchemaStore` — no parent-form coupling,
no dirty-registry dependency. It provides `INLINE_CREATE_DEPTH` itself.
2. **`embedded` mode reads an EMPTY query, deliberately** (`DynamicForm.vue:655-661`):
an embedded form is mounted over the host's page, so honouring `route.query`
would pre-fill the nested entity from the host's params and auto-link it to the
host's peer. This means the modal flow **cannot** reuse the URL channel;
pre-link and template must arrive as explicit props. Reusing the URL channel
here would reintroduce exactly the bug that comment prevents.
3. **Every type already has a view, configured or not.** `handleV1Views`
(`views_handler.go:513-516`) looks a view up by `entry.type` and, on a miss,
falls back to `buildDefaultViewConfig` through the identical pipeline. The
synthesizer emits one traverse rule plus one `display: cards` section per
relation — outgoing as `out_<name>`, incoming as `in_<name>`, symmetric
relations skipped — and **never** emits `Recursive`, `MaxDepth`, or a
non-`entry` `From` (`default_view.go:73-120`). So the ambiguous shapes exist
only in hand-authored `views:` config, and for the majority case (no `views:`
entry) the section-to-relation mapping is unambiguously 1:1, so the ambiguous
shapes are rarer than feared. Note this does NOT make the feature work without
config: the affordance is opt-in (see below), and a synthesized view carries no
`create:` blocks. It means only that an operator who authors a `views:` entry
will usually find an unambiguous relation to attach a button to.

**Prior-art decision reused:** TKT-OMUD56 settled that the create *offer* is
derived (permission AND a resolvable form), never declared, and that sending the
resolved form id makes presence-in-the-map the affordance so the client does no
permission arithmetic. This ticket follows that rule rather than re-litigating
it.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### 1. Config (`internal/dataentryconfig/config.go`)

```go
// SectionCreate declares HOW a create-related affordance behaves on a section.
// WHICH types are offered stays derived (permission + a resolvable form), per
// TKT-OMUD56 — this struct never names a type as an allowlist entry.
type SectionCreate struct {
    In    []string                       `yaml:"in,omitempty"`    // header|section; default [section]
    Flow  string                         `yaml:"flow,omitempty"`  // modal|page; default modal
    Types map[string]SectionCreateTarget `yaml:"types,omitempty"` // per-type overrides
}

type SectionCreateTarget struct {
    Template string `yaml:"template,omitempty"`
}
```

Added to `ViewSection` as `Create *SectionCreate \`yaml:"create,omitempty"\``.

**The affordance is OPT-IN, and that is load-bearing.** TKT-651W deliberately
removed create buttons from this exact surface, implementing FEAT-K111 "Entity
views are strictly read-only", because "editing the graph from inside a read view
blurs the line between viewing and editing". That invariant stands. This ticket
does not reverse it; it **narrows** it to "read-only unless an operator explicitly
asks otherwise".

So a section renders a button only when it carries an explicit `create:` block.
A section without one behaves exactly as today: no affordance, nothing on the
wire. `create: {}` is the one-line opt-in taking every default.

This makes `Create *SectionCreate` a plain pointer — absent means off, present
means on — so **no bool-or-mapping union and no custom unmarshal are needed**.
`create: false` is unnecessary (absence already expresses it) and is refused at
load so there is one spelling per meaning, following the `DarkMode` precedent's
posture if not its shape.

Consequences of opting in rather than defaulting on:

- **Existing deployments are untouched on upgrade.** No new buttons appear anywhere
  until an operator edits config. The upgrade-visibility risk disappears.
- **`buildDefaultViewConfig` types get nothing by default.** A deployment with no
  `views:` entry has no `create:` blocks either, so the synthesized view stays
  read-only. An operator wanting buttons must author a `views:` entry. That is a
  real cost of this choice and is accepted: the invariant's default matters more
  than the convenience.
- **The guard test narrows rather than inverts.** `TestV1Views_NoAddOrLinkInfoOnSections`
  keeps asserting absence for every section with no `create:` block — all five of
  its existing sub-cases stay valid and stay green — and gains cases asserting
  presence only where a block is configured. It remains a live guard against the
  accidental re-bleed RR-R8X6 predicted.
- **The three enforcement comments are updated, not deleted** (`responses.go:1088-1092`,
  `sections.go:429-435`, and the guard test's own header), each stating the narrowed
  rule and citing both TKT-651W and this ticket, so the next reader finds the history
  this plan initially missed.

`Flow` is relation-wide because a flow is a UI choice and is type-independent;
one type opening a modal while its sibling navigates would read as a bug.
`Types` is keyed by entity type because a template variant only exists per type
— a heterogeneous relation (`to: [task, bug]`) has no single meaningful
template.

**Why `ViewSection` and not `ViewTraverse`.** `in` and `flow` are placement and
UI-flow choices, which are genuinely section properties, whereas `ViewTraverse`
(`config.go:1354-1362`) is a pure data-collection rule with no presentation keys
at all — putting `create:` there would be the real precedent violation. (The
`ParentColumns`/`ChildColumns` analogy justifies only the per-type *map shape*,
not the choice of host struct; it is presentation keyed by type and says nothing
about affordances.)

**The unresolved case: two sections sharing one `Source`.** Nothing in
`ViewConfig` forbids it, and one section is fed by exactly one rule (the loop
`break`s on first match, `sections.go:527`), so the many-to-one direction is the
live one. Two sections over one bucket may then declare conflicting
`types.<type>.template` values, and the header union dedups the relation to one
entry — with whose template? **Decision: a load error.** Two sections over one
bucket declaring different `create.types` for the same type is a config
contradiction, and the alternative (last-wins) would depend on section order,
which is a layout concern nobody expects to change behaviour. `in`/`flow` may
differ freely between such sections since they are per-section by nature; only
the header union needs a rule, and it takes the entry from the first section
that opted in.

Load-time validation (allowlist, not blocklist): `flow` ∈ {modal, page}; each
`in` ∈ {header, section}; each `types` key must be a type the section's
originating relation can actually reach; each `template` must exist for that
type. An explicit `create:` mapping on a section whose relation cannot be
determined is a load error, not a silent no-op — silently dropping it would
leave the operator debugging a missing button. An *absent* `create:` on such a
section is fine and simply yields no button, since the operator asserted
nothing.

### 2. Relation threading (`internal/dataentry/sections.go`)

`resolveSectionButtonsWithTraverse` already finds the relation by matching
`rule.CollectAs == sec.Source && rule.From == "entry"`. Generalize it to run on
the `_views` path and to record the resolved relation on the section.

**The resolver has NO ACL check today — this is the single largest piece of new
work, not a rewiring.** Verified: the body (`sections.go:436-528`) contains zero
calls to `computeCollectionActions` or anything in `acl`. It derives targets from
`createFormForType(et) != ""` alone, and `createFormForType`
(`views_handler.go:773-795`) is a pure config lookup over `s.Cfg.Forms` with no
principal involved. So the affordance currently means "a form exists", not "you
may create one".

Three consequences the plan must own:

1. **AC1 and AC10 rest on a call that does not exist yet.** Adding
   `computeCollectionActions` to the resolver is the mitigation, and it must be
   written, not merely invoked from a new caller.
2. **`viewsHandler` must reach `affordanceService`.** Check the wiring before
   implementation; if it does not already hold one, that is a constructor change
   subject to the nil-required-fields rule in CLAUDE.md.
3. **`_sidepanel` gains a gate it does not have today.** That is a behaviour
   change on a shipped surface: a principal who may not create type X currently
   sees an Add button there and will stop seeing it. This is a **fix**, not a
   regression — the button led to a form whose POST would be refused anyway — but
   it changes existing behaviour and its tests, and it belongs in the release note
   beside the narrowed read-only invariant.

**`LinkInfo` must be split out, not carried along.** The same function
unconditionally builds `SectionLinkInfo` whenever `len(candidateTypes) > 0`
(`sections.go:517-525`) — no form check, no permission check. "Link existing" is
explicitly OUT of scope, so generalizing the function wholesale would ship an
ungated link affordance onto `_views`. Either split the function so `_views` gets
only the add path, or gate both. Do not let it ride along.

Sections with no single originating relation — `recursive: true`, a rule whose
`from:` is another bucket rather than `entry`, or `source: entry` — **degrade to
no button**. Guessing a relation would link the new entity to the wrong peer,
which is worse than the absent affordance.

**Worlds are NOT inherited — the create must carry one explicitly.** The two
paths differ here and the plan must not paper over it. `_sidepanel` pins
`defaultViewWorld()` (`sections.go:417-422`: a surface must DECLARE its world),
but `_views` is **the one surface that opts into worlds**
(`views_handler.go:523-527`, `viewWorldFromRequest`). So extending button
resolution to `_views` puts it on a world-capable surface for the first time,
and the world the user is viewing decides which face the new entity must land in
(`worlds.<name>.create`; `DynamicForm.vue:1400-1404` — a faced type has no
default row to fall back to, so omitting the world is a refusal, BUG-HC6I2T).

- **Page flow**: correct for free. `useWorld` reads `?world=` off the route and
  `DynamicForm` already carries it into the create payload and the redirect
  (`:1404`, `:1504-1514`).
- **Modal flow**: the risk. The embedded form reads an empty *query* for pre-fill,
  but `useWorld` reads the **route**, not that overlay — and here the host's
  world is exactly the right one, unlike the host's `link_*` params. The
  distinction is subtle enough to be broken by accident, so the world must be
  passed explicitly as a prop and a test must pin that a modal create from a
  world-bound detail page lands at that world's face.

This also means `resolveSectionButtonsWithTraverse` must run under the request's
world, not the default, or the derived targets can describe rows the user is not
looking at.

### 3. Wire (`internal/apiwire/v1/responses.go`)

**Do NOT reuse `ViewAddInfo`.** Its doc comment (`responses.go:1088-1092`)
explicitly forbids it: "Do not reach for this type from a new view-related
response." RR-R8X6 predicted that its misleading `View` prefix would tempt exactly
this reuse, and it did. Define **new types** (`ViewSectionCreate` /
`ViewSectionCreateTarget`) carrying relation, linkAs, peerId, flow, and per-target
`{entityType, formId, label, template}`.

Carrier: a new `omitempty` field on `ViewSection` for the per-section affordance,
and one on `ViewResponse` for the header union. `ViewResponse` is a flat 4-field
struct, so this stays far under plimsoll's 20-field cap, and `omitempty` keeps
responses byte-identical for sections that did not opt in — which is what lets the
guard test's existing cases stay green unchanged.

**Receiver matters for plimsoll.** `App` is pinned at
`//plimsoll:max-methods=89` / `max-exported-methods=22` (`app.go:192-193`), so a
new method there fails CI. Header-union assembly goes on `viewsHandler`, which
carries no directive.

The header menu is assembled **server-side**, deduped by relation, in section
order, so the SPA does not reimplement the aggregation and the Go handler test can
assert a fixed payload.

### 4. Frontend

- One `SectionCreateButton.vue` used by all six `display:` branches, rather than six
copies. Renders a direct button for one target and a menu for several.
- A header menu component fed by the server-assembled union.
- **Page flow**: `router.push('/form/:formId')` with `link_relation`, `link_peer`,
`link_as`, `return_to`, and a new `template` param.
- **Modal flow**: mount `InlineCreateFormModal` with new props for the pre-link
triple and the template. `DynamicForm` gains matching props that feed the same
code paths the URL params feed, so the two flows share one implementation of
pre-link and template application, not two.
- On modal success, refetch the view. **`loadView()` is a full-view refetch**
  (`EntityDetail.vue:666-698`) — it replaces `viewData` wholesale and there is no
  per-section fetch. So "in place" means "no route change", not a cheap partial
  update; the AC must not imply otherwise.

**Query budget.** `internal/dataentry/querybudget_test.go:207` already has
`TestQueryBudget_ViewTableSectionIsSizeIndependent`. Button resolution adds
per-section work to that exact path, so extend the fixture to assert the count
stays flat with `create:` enabled — CLAUDE.md requires a budget test for new read
paths, and here it is nearly free.

Two costs to fix rather than discover: `createFormForType`
(`views_handler.go:772-790`) builds and `natsort`s the **full form-id list on every
call** — per section, per target type — and the new `AuthorizeWrite` runs per
candidate type per section, repeating for types reachable by several relations.
Memoize both per request, keyed by entity type.

### 5. Side-panel fix

Change `SidePanel.vue:74-79` to emit `link_relation`/`link_peer`/`link_as` — the
names `DynamicForm` actually reads. A test asserts the button's target URL
carries params the form consumes, so the two cannot drift apart again silently.

**Alternatives considered:**

- *Fully-declared per-section targets* (operator lists type + form + template each).
Rejected: duplicates the derivation and drifts from ACL and registered forms —
precisely the failure TKT-OMUD56's derive rule was adopted to avoid.
- *Flat `template:` restricted to single-type relations.* Rejected: leaves
heterogeneous relations with no template support, and they are common (15 of 47
relations in rela's own schema declare more than one `to:` type).
- *Header menu declared independently at view level.* Rejected: a section knows its
relation only through its `traverse:` rule, so a view-level list would need to
re-derive the relation-to-section mapping. Aggregating from sections keeps one
source of truth.
- *Extending `_actions`.* Rejected: closed verb set, `panic` on unknown verbs, and a
lint invariant confining `acl.WriteRequest{Op:` construction to
`affordances.go`. It has no vocabulary for "create type X via relation Y".
- *Reusing the URL channel for the modal flow.* Rejected: `embedded` deliberately
reads an empty query (`DynamicForm.vue:655`); reusing it would reintroduce the
host-param leakage that comment exists to prevent.

**Files to modify:**

- `internal/dataentryconfig/config.go` — `SectionCreate`, `SectionCreateTarget`, `ViewSection.Create`
- `internal/dataentryconfig/` validation — load-time checks
- `internal/dataentry/sections.go` — generalize button resolution; record relation
- `internal/dataentry/views_handler.go` — call resolution on the `_views` path; assemble header union
- `internal/apiwire/v1/responses.go` — wire fields
- `frontend/src/types/config.ts`, `frontend/src/types/entity.ts` — TS types
- `frontend/src/components/entity/EntityDetail.vue` — section buttons + header menu
- `frontend/src/components/entity/SectionCreateButton.vue` — new
- `frontend/src/components/forms/InlineCreateFormModal.vue` — pre-link + template props
- `frontend/src/components/forms/DynamicForm.vue` — prop channel mirroring the URL channel
- `frontend/src/components/forms/SidePanel.vue` — param-name fix
- `docs/data-entry.md` — § Sections (~line 1526)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
| --- | --- | --- | --- |
| `create.flow` | operator config | allowlist {modal, page} | config load error |
| `create.in[]` | operator config | allowlist {header, section} | config load error |
| `create.types` keys | operator config | must be reachable by the section's relation | config load error |
| `create.types.*.template` | operator config | must exist for that type | config load error |
| `link_relation`/`link_peer`/`link_as` | URL query (user-controllable) | server re-authorizes on write; `link_as` ∈ {from, to} | write refused |
| `template` | URL query (user-controllable) | client-side selection only; server applies nothing | ignored if unknown |
| `return_to` | URL query (user-controllable) | existing `readReturnTo` open-redirect guard | dropped |
| `formId` | server-resolved | never user-supplied (existing rule) | n/a |

**Security-Sensitive Operations:**

1. **Create authorization.** The button is derived from `computeCollectionActions`
— the same call the list handler uses, so the affordance cannot diverge from the
write path. The button is an affordance, never a grant: a forged POST is
authorized independently by the write path. AC10 tests exactly this.
2. **Relation authorization — and the limit of it.** The edge create is gated by
`RelationOpCreate` independently of entity-create permission; the two are
orthogonal (TKT-OMUD56) and are AND-ed. Verified on both directions, but by
**two different functions**: `link_as: from` posts a separate relation
(`handleV1CreateRelation` → `validateRelationOp`, `write_handler.go:947`), while
`link_as: to` rides the create body (`validateRelationsModernAffordances` →
the same `validateRelationOp`). Two implementations of one rule can drift, so
AC10 must assert refusal on **both** directions against the same principal.

The gate is **default-permissive for undeclared relation types**
(`affordances.go:608-611`: no verdict entry returns nil). So the real rule is
`entity-create ∧ (relation-verdict ∨ none-declared) ∧ manager-level ACL`, not an
unconditional relation check. An AC10 written against a schema with no relation
verdicts would pass while exercising nothing; the test must declare a
non-creatable verdict.

3. **`link_*` is user-controlled in the page flow, and provenance is NOT
verified.** The server computes `AddInfo` and the SPA serializes it into a URL —
but the server never sees its own `AddInfo` again. There is no signature or
re-resolution, so a user may edit the triple to any relation and any peer. The
write path re-authorizes the *operation* (`gateRead(peer)` ∧ `RelationOpCreate`
∧ metamodel endpoint-type validity) but cannot re-validate the *provenance*.

Combined with default-permissive verdicts, that means: on a deployment declaring
no relation verdicts, read access to an entity is sufficient to attach any
metamodel-valid edge to it. **This is pre-existing** — the same is true of every
existing create form that accepts `link_*` — and this ticket does not widen it,
but the plan must not claim server-sent provenance as a security property. A
later reader would build on a guarantee that was never real. Documented as a
boundary; operators relying on relation-level restriction must declare relation
verdicts. Re-deriving server-side (carrying section identity instead of the raw
triple) is the option that would make the claim true, and is deliberately not
taken here.
4. **`create` implies no `read`** (`internal/acl/policy.go:239-243`). A principal may
create a type it cannot read, so after a modal create the refetch may
legitimately not return the new row. That must degrade to showing the id, never
to an error claiming the create failed (the TKT-OMUD56 AC11 precedent).
5. **The pre-link fails open to silence today, and this ticket must close that.**
`DynamicForm.vue:1417-1421` resolves the peer's type client-side via
`getTypeFromId(peer)` and, when the prefix does not resolve, skips the link
**with no `else` branch** — producing a created, unlinked entity and no error on
the navigate path. AC5 makes the pre-link load-bearing ("the user never links
manually"), so an unresolvable peer must become a visible failure rather than a
skipped step. This is in scope: it is the same pre-link contract the side-panel
param fix is about, and shipping buttons that rely on it while it fails silently
would multiply orphans rather than prevent them.

6. **No config concealment.** Per CLAUDE.md, view and section names are not secret. A
`create:` block naming an unreachable type is a load error with a clear message,
not a silently dropped button — the operator debugging it is the audience.
7. **Open redirect.** `return_to` keeps the existing guard; this ticket adds no new
path that bypasses `readReturnTo`.
8. **No new external I/O**, no file access, no crypto, no `Tx`-scoped work.

Error handling surfaces the missing permission for a config-declared capability
(useful to the operator) while entity-level 404s stay uniform — the distinction
CLAUDE.md draws between config keys and entity ids.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Level |
| --- | --- | --- |
| 1 | Section WITH a `create:` block yields the affordance on the wire; a section without one yields nothing (the existing guard test's five cases stay green); a principal lacking create for every target yields nothing | Go handler test, two principals |
| 2 | Relation with one `to:` type renders a direct button; two types render a picker | Vue component test |
| 3 | Modal flow opens embedded form, emits created, section shows the new row, `route` unchanged | Vue test + e2e |
| 4 | Page flow navigates with the four params and returns to the originator with the row present | e2e |
| 5 | `link_as: to` and `link_as: from` both produce the correct edge direction | Go integration test on the write path |
| 6 | `types.task.template: bugfix` preselects that variant; a type with no entry uses the form default | Vue test |
| 7 | Header menu is the union of opted-in sections; absent when none opted in | Go handler test |
| 8 | Side-panel Add target URL carries params `DynamicForm` reads — **fails before the fix** | Vue test |
| 9 | Each invalid `create:` shape is refused at load with a message naming the offending key | Go table test |
| 10 | Contract: no button for a type the principal cannot create; the forged POST 403s; edge refusal asserted on **both** `link_as: to` and `link_as: from` against a schema declaring a **non-creatable relation verdict** (a schema with no verdicts is default-permissive and would pass vacuously) | Go contract test |
| 11 | An unresolvable `link_peer` surfaces a visible failure, never a silently unlinked entity | Vue test + Go |
| 12 | Create from a **non-default world** lands the new entity at that world's face, on both flows | Go + Vue test |
| 13 | The `_sidepanel` Add button disappears for a principal who may not create the target type (the new gate; a declared behaviour change) | Go handler test |
| 14 | Query budget: `_views` count is unchanged at 10 and 50 rows with `create:` enabled | `querybudget_test.go` |

AC1 is split deliberately: a Go handler test asserts the **wire field**, a Vue
component test asserts the **button renders from it**. Conflating them would make
AC1 untestable at either level, and the wire half is exactly what the existing
guard test asserts to be absent.

**Integration approach:** AC5 and AC10 exercise the real write path against the
real ACL instance, following the `affordances_contract_test.go` pattern
(affordance false ⇒ write 403) rather than asserting on a mock. AC3 and AC4 get
e2e coverage in `e2e/tests/` because the flows span router, form and section
refresh — the seam where a unit test would pass while the feature is broken.

**Edge Cases:**

- Section fed by a `recursive: true` rule → no button (no single relation).
- Section whose traverse `from:` is another bucket, not `entry` → no button.
- `source: entry` (the properties section) → no button.
- Relation reaching types where the principal may create *some* → button offers only those.
- Relation reaching types where *none* has a form → no button.
- Zero related items (empty section) → button still renders; that is the main case for it.
- Modal create of a type the principal cannot read → show the id, not an error (AC11 precedent).
- Template variant deleted from disk after config load → form falls back to its default.
- Two sections in one view sharing the same relation → header menu lists it once,
deduped by relation, in section order.
- `create.types` naming a type the relation reaches but for which no form resolves →
load error. The validation checks reachability and template existence; it must also
check form resolvability, or the config passes load and silently produces no button.
- Modal create of a type the principal may create but not read → the full-view
refetch cannot show the new row, and unlike TKT-OMUD56's widgets there is no cache
to seed from the POST response. AC3 carries an explicit exception for this case:
a toast naming the created id, not a silent no-op and not an error.
- Entity type with no `views:` entry → the synthesized view's relation sections each
get a button, since every synthesized rule is `From: entry` with one relation.
- Section with no `create:` block → no button, nothing on the wire, section excluded
from the header union. This is the DEFAULT and is what the existing guard test
asserts.
- `create: false` / `create: true` → load error. Absence already means off, so a
second spelling for it would be one more thing to explain forever.
- Modal create from a world-bound detail page → new entity lands at that world's
face, not the default.
- A type whose only relations are symmetric → the synthesizer skips the incoming
duplicate (`default_view.go:94`), so an operator authoring `create:` for it gets
one section, one button.
- Concurrent create of the same relation twice (double-click) → the existing
`isSaving()` guard in `InlineCreateFormModal.requestClose` covers the close
race; submit-side double-fire is already handled by `DynamicForm`.

**Negative Tests:**

- `flow: dialog` → load error naming `flow`.
- `in: [sidebar]` → load error naming `in`.
- `create:` mapping on a section whose relation cannot be resolved → load error.
- `create: false`, `create: true`, or any scalar → load error; `create:` is a
mapping and absence is how a section opts out.
- `types: {nonexistent-type: ...}` on a relation that cannot reach it → load error.
- `types: {task: {template: no-such-variant}}` → load error.
- POST with a forged `link_relation` the principal may not create an edge for → 403.
- POST creating a type the principal may not create → 403 regardless of any button.
- `return_to=//evil.com` → guard drops it; user lands on the entity instead.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
| --- | --- |
| Affordance/write divergence — a button for a refused action | Derive from `computeCollectionActions`, the same call the list handler uses; AC10 contract test pins it |
| `EntityDetail.vue` is a 3072-line six-branch render; buttons could be copied six times | One `SectionCreateButton.vue` in the section header, above the branch |
| Relation threading guesses wrong for recursive/multi-hop sections | Degrade to no button; explicit tests for each shape |
| Two pre-link implementations (URL for page, props for modal) drift | Props feed the same code paths as the URL params; one implementation, two entry points |
| Side-panel param mismatch recurs | Test asserts the emitted URL carries params the form reads |
| Scope creep into FEAT-KQ45P's detail-screen unification | Explicitly out of scope; no refactor of the branch structure |
| Re-bleeding the read-only invariant beyond the opt-in (RR-0A83G4) | The guard test narrows rather than inverts: it keeps asserting absence for every section with no `create:` block, so an accidental widening stays red |
| Operators with no `views:` entry cannot get buttons without authoring one | Accepted cost of honouring FEAT-K111's default; documented in `docs/data-entry.md` |

**Effort:** l — spans config schema plus validation, Go handler, wire types,
three Vue components, docs, and e2e. Mostly wiring existing mechanisms, which is
why it is not xl, but it crosses every layer.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — § Sections (~line 1526) gains the `create:` block: keys,
defaults, the per-type `types:` map, the two flows, and the derivation rule (why
there is no per-type allowlist). A worked heterogeneous-relation example.
- [x] ~~docs/metamodel.md~~ (N/A: this is data-entry config, not the metamodel)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI change)
- [x] `CLAUDE.md` — the read-only-view invariant is now conditional (read-only unless a section opts in). Worth a line so the next reader does not rediscover TKT-651W the hard way, as this plan did.
- [x] ~~README.md~~ (N/A: not a project-level change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** two independent reviewers (security + design), each
verifying claims against the code. Ten findings, all addressed in the plan above.

| ID | Severity | Finding |
| --- | --- | --- |
| RR-0A83G4 | critical | Plan reversed TKT-651W's read-only-view invariant without knowing it existed; a guard test and three comments enforce it |
| RR-YDYLZ5 | critical | The section button resolver has no ACL gate at all; the plan's central mitigation cited a call that does not exist |
| RR-V32AZK | significant | Plan claimed server-sent provenance for the relation triple; the page flow round-trips it through a user-editable URL |
| RR-8SP2UG | significant | Pre-link fails open to silence when the peer id prefix does not resolve |
| RR-XH2DD3 | significant | Extending resolution to `_views` puts it on the world-capable surface for the first time; worlds and faces were unaddressed |
| RR-DZJACK | significant | Generalizing the resolver wholesale would ship an ungated link-existing affordance, which is out of scope |
| RR-YNZBKN | significant | AC10 would have passed vacuously: default-permissive relation verdicts, and only one of two code paths tested |
| RR-4IB3OQ | significant | Full-view refetch mislabelled, no query-budget test, repeated form-list sorting per section |
| RR-AM48JH | significant | `ViewAddInfo` is the forbidden carrier; `App` is at its plimsoll method cap |
| RR-19AU91 | minor | Bool-or-mapping YAML union had an in-package precedent (`DarkMode`) and needs four methods, not one |

The two critical findings changed the design rather than merely annotating it.
RR-0A83G4 was escalated to the user, who chose to honour the invariant — so the
affordance became opt-in and the plan's default-on premise was withdrawn. RR-YDYLZ5
moved the authorization from "already exists, just call it" to the largest single
piece of new work.
