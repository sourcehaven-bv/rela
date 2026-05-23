---
id: PLAN-C5DH
type: planning-checklist
title: 'Planning: _fields / _relations wire shape + SPA renderer (stub verdict source)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**IN:**

- Wire-shape extension on `V1Entity` and per-entity GET response:
  - `_fields: Record<string, { writable: boolean; options?: Record<string, boolean> }>`
  - `_relations: Record<string, { creatable: boolean; removable: boolean; fields?: Record<string, { writable: boolean }> }>`
- **Hidden semantics (revised per design-review F1):**
  - Server omits the property from `properties` AND from `_fields`.
  - **SPA actively filters its config-driven field list against `_fields`**: a
    form field declared in `data-entry.yaml` is rendered only if either
    (a) its property name appears in `entity._fields`, or
    (b) its property name appears in `entity.properties`.
    This makes "hidden = omitted" actually hide the input.
  - Create mode is unrestricted (see F10 resolution below).
- Stub `FieldVerdictResolver` interface with two profiles:
  - `none` (default) — everything writable/visible/creatable/removable
  - `demo` — fixture against the `ticket` type exercising every code path
    (renamed from `triager-demo` per F9)
- Profile resolution: `RELA_AFFORDANCE_PROFILE` env var parsed in `cmd/rela-server/main.go`,
  passed as a constructor argument to `dataentry.New` (per F7 — tests pass the
  resolver directly without env-var manipulation).
- SPA consumption in `DynamicForm`, `FieldRenderer`, `RelationCards`:
  - `_fields[name].writable=false` → input rendered with existing `readonly` prop
  - field absent from `_fields` and from `properties` → form skips rendering
    (NEW filter in `DynamicForm.vue`'s `fields` computed)
  - `_fields[name].options.<v>=false` → option absent from `<select>`
  - `_relations[type].creatable=false` → `+ Add` button hidden
  - `_relations[type].removable=false` → per-link `x` hidden for **all** links
    of that type (per-relation-type uniform, per F5)
  - `_relations[type].fields[name].writable=false` → meta-field input disabled
    in `RelationCards`
- Wire-vs-policy parity at the write path (revised round 2 per F12: drop
  friendly-drop semantics — `useAutoSave` already does no-op suppression
  client-side via `lastSeenServer`, so the matching-value branch is dead code
  for any real SPA path):
  - **PATCH with hidden field present** (set or unset) → 403
  - **PATCH with read-only field present** (set or unset, any value) → 403
  - **PATCH with disallowed enum value** → 403
  - **PATCH with unknown field** (declared neither in metamodel nor known by
    resolver) → 403, structurally identical to hidden (F8 side-channel closure)
  - **POST/DELETE relation with `creatable=false` / `removable=false`** → 403
  - **PATCH that creates or deletes a relation via the unified path with a
    `creatable=false` / `removable=false` verdict** → 403
  - **PATCH with non-writable relation-meta field present** (set via `Meta` or
    listed in `MetaUnset`, any value) → 403

**OUT (follow-ups):**

- Real verdict source — `acl.yaml` schema for field/option/relation-meta grants.
  Predicate-engine ticket owns this and replaces the stub directly.
- Type-default + per-entity-override compression of `_fields` for list views
- List-query field-level read enforcement (only per-entity GET emits the new fields)
- Cache invalidation beyond entity ETags
- Masked-value rendering
- Per-link affordances (different verdicts for different individual links)
- Audit-log `denied-read` events for hidden fields (existing `denied-write` covers
  the parity case)
- **Create-mode affordances** (F10 resolution): create-mode SPA renders all
  fields normally; the stub doesn't gate creates. Predicate ticket addresses
  create-mode separately (it needs collection-level affordances on
  `V1ListResponse`, which is an additional wire-shape extension).
- **List-typed enum option-filter** (F15): the per-element check for
  `list: true` enums (e.g. `tags`) is deferred. The wire shape supports it
  (the `options` map is unambiguous for list-typed too), but the demo profile
  doesn't exercise this and the PATCH validator only walks scalar enums in
  v1. Predicate ticket inherits the wire shape; the validator extension is a
  small, additive change.
- **Audit-op separation for affordance denials** (F21): v1 keeps reusing the
  existing `denied-write` op, distinguished by a stable `affordance:<reason>:<field>`
  prefix in the `reason` field (vs `acl:...` for ACL denials). No new audit
  op constant for this stub — predicate ticket re-evaluates when affordance
  becomes a real policy surface.

### Branching decision (F2)

Branch off **`develop` directly**, not off `feat/acl-affordances` (PR #779).
Rationale: stacking on #779 reintroduces exactly the PR-gates-PR problem that
"single combined ticket" was meant to avoid. If #779 merges first, this branch
gets a clean rebase. If #779 doesn't merge first, this PR reshapes
`V1Entity.Actions` from `*V1Actions` to `map[string]bool` itself (~80 LOC, the
phase-1 work) and the conflict resolution at merge time is straightforward.
The plan budget includes this contingency.

### Demo profile fixture (F4 resolution)

The rela project's actual `ticket` properties are `title`, `kind`, `priority`,
`effort`, `tags`, `status`. The `demo` profile applies to type `ticket`:

| Path | Verdict | Exercises |
|---|---|---|
| `title` | writable (default) | Sanity-check writable wire entry |
| `kind` | read-only | Read-only field code path; PATCH 403 on change |
| `priority` | hidden | Omission from `properties` + `_fields`; SPA field-filter |
| `effort` | writable, options `{xs:T, s:T, m:T, l:F, xl:F}` | Option filter; PATCH 403 on disallowed option |
| `status` | writable, options `{backlog:T, ready:T, planning:T, in-progress:T, review:T, done:F}` | Two enum demonstrations (effort+status) |
| relation `affects` | `creatable: false` | Relation create 403 |
| relation `implements` | `removable: false` | Relation delete 403 |
| relation `has-planning` | meta-field `<chosen>.writable=false` | Relation-meta PATCH 403 |

The relation-meta target needs verification against `tickets/metamodel.yaml`
during implementation; the fixture chooses whichever relation type actually
has a writable meta property (likely none currently — may need to pick a
different entity type for the meta demonstration, or add a stub meta on
`has-planning`). Documented in the implementation checklist as a finalization
step.

**Acceptance Criteria:**

1. **AC1 — Default profile is non-disruptive (revised per F13: sparse emission).**
   With `RELA_AFFORDANCE_PROFILE` unset (or `none`), per-entity GET responses
   contain `_fields: {}` and `_relations: {}` (present but empty), and **no
   properties are omitted from `properties`**. Sparse emission: only deviations
   from default appear in the maps. Existing SPA behavior is indistinguishable
   from today; the F1 SPA filter uses `entity.properties` (not `_fields`) as
   the field-visibility signal, so an empty `_fields` map renders the form
   unchanged.
   - Test: `TestAppRouter_PerEntityGet_NoneProfile` asserts `_fields == {}`,
     `_relations == {}`, and `properties` matches the entity's full property
     set.

2. **AC2 — `demo` profile produces the expected fixture verdicts (sparse).**
   With `RELA_AFFORDANCE_PROFILE=demo`, a GET for a `ticket` entity returns
   ONLY the entries that deviate from default:
   - `_fields.kind = {writable: false}` (the only field with a writable override)
   - `properties.priority` absent; `_fields.priority` absent (hidden — emitted
     in neither map; verified by SPA filter's "in properties OR in _fields" check)
   - `_fields.effort = {options: {l: false, xl: false}}` (only the false
     entries appear; sparse within `options` too — `xs:true`, `s:true`,
     `m:true` are implied by absence)
   - `_fields.status = {options: {done: false}}` (only the false entry)
   - `_relations.affects = {creatable: false}` (only the deviation)
   - `_relations.implements = {removable: false}`
   - `_relations.<chosen-relation>.fields.<chosen-meta> = {writable: false}`
     (relation type and meta-field picked during implementation; see "Demo
     profile fixture" caveat)
   - All other fields and relations: ABSENT from `_fields` / `_relations`
     entirely. The SPA's "no entry = default" path handles them.
   - Test: `TestAppRouter_PerEntityGet_Demo` asserts each entry present and
     the absence of all other entries.

3. **AC3 — Hidden fields are bidirectionally invisible (F8 + F16).**
   - PATCH `{properties: {priority: "high"}}` under `demo` → 403 with
     `{rule: "field-affordance:hidden", field: "priority"}`. Audit log records.
   - PATCH `{properties_unset: ["priority"]}` under `demo` → 403 (unset on a
     hidden field is a write).
   - PATCH that sets a TRULY unknown field (`bogus_field`) under any profile →
     403, structurally identical response shape (`{rule: "field-affordance:hidden", field: "bogus_field"}`).
     This closes the F8 side-channel: hidden and unknown produce byte-equivalent
     errors, so an attacker cannot distinguish "hidden from me" from "doesn't
     exist." It also changes today's behavior — rela currently silently merges
     unknown fields; this AC ships the new rejection.
   - Test: `TestAppRouter_PatchHiddenField_Forbidden` (set + unset subtests).
   - Test: `TestAppRouter_PatchUnknownField_Forbidden` asserts byte-equivalence
     of the 403 response with the hidden case (modulo the field name in `field`).

4. **AC4 — Read-only fields reject writes (F12 revision: simplified to strict 403).**
   `useAutoSave` performs no-op suppression client-side (`lastSeenServer`
   check in `useAutoSave.ts`), so no real SPA path produces a same-value
   PATCH; the friendly-drop branch from round-1 F3 was dead code.
   - PATCH `{properties: {kind: "different"}}` under `demo` → 403 with
     `{rule: "field-affordance:read-only", field: "kind"}`; file unchanged.
   - PATCH `{properties: {kind: <current>}}` under `demo` → 403 (same rule).
     Strictly equivalent to the different-value case; SPA won't emit this
     PATCH naturally.
   - PATCH `{properties: {kind: "different", title: "new"}}` under `demo` →
     403; whole PATCH rejected, including `title`. No partial application.
   - PATCH `{properties_unset: ["kind"]}` under `demo` → 403 (unset is a
     write).
   - Test: `TestAppRouter_PatchReadOnlyField` with four subtests covering each
     case.

5. **AC5 — Filtered enum options reject writes.** PATCH `{properties: {status: "done"}}`
   under `demo` returns 403. PATCH `{properties: {status: "review"}}` succeeds.
   PATCH `{properties: {effort: "l"}}` returns 403; `{effort: "m"}` succeeds.
   - Test: `TestAppRouter_PatchFilteredOption_Forbidden`.

6. **AC6 — Relation create/remove gates at EVERY relation-write endpoint
   (F5 + F14).** The plan's round-1 "single chokepoint" claim was wrong.
   Three endpoints write relations:
   - `handleV1CreateRelation` (api_v1.go:863) — per-relation POST
   - `handleV1DeleteRelation` (api_v1.go:960) — per-relation DELETE
   - `handleV1UpdateEntity` (api_v1.go:542) — unified PATCH with `relations`
     field (modern reconciler adds/removes relations)
   Each must consult `RelationVerdicts.Types[type]` before writing. The
   shared validator function lives in `affordances.go` (close to the
   resolver wiring); each handler invokes it before passing the request to
   `entityManager.{CreateRelation,DeleteRelation}` / the modern reconciler.
   Behavior:
   - POST `/api/v1/entities/ticket/T1/relations/affects` (add) under `demo` →
     403 with `{rule: "relation-affordance:not-creatable", type: "affects"}`.
     No link identifier (gate is per-type, not per-link).
   - DELETE `/api/v1/entities/ticket/T1/relations/implements/T2` under
     `demo` → 403 with `{rule: "relation-affordance:not-removable", type: "implements"}`.
   - PATCH `/api/v1/entities/ticket/T1` with `relations: {affects: {data: [...]}}`
     that ADDS an `affects` edge under `demo` → 403 same shape.
   - PATCH that REMOVES an `implements` edge under `demo` → 403 same shape.
   - Other relation types via any endpoint succeed.
   - Test: `TestAppRouter_RelationGates_Forbidden` covers all four endpoint ×
     verb combinations.

7. **AC7 — Relation meta-field gates (F12 simplification + F16 unset).**
   - PATCH unified body with `relations: {<type>: {data: [{id: ..., meta: {<non-writable>: "v"}}]}}`
     under `demo` → 403, `{rule: "relation-affordance:meta-read-only", type: ..., field: ...}`.
     Same value or different value — strict 403 (F12).
   - PATCH unified body with `meta_unset: ["<non-writable-meta>"]` → 403
     (same rule).
   - PATCH unified body that updates a writable meta-field on the same
     relation type → 200.
   - POST `/api/v1/entities/.../relations/<type>` with `meta: {<non-writable>: "v"}` →
     403 (same rule).
   - Inspection point on the unified path: `V1RelationsUpdate.Meta` and
     `MetaUnset` in `relations_v1_wire.go` (line 58). Per-relation POST
     inspection point: the `Meta` field in `handleV1CreateRelation`'s
     local request struct (api_v1.go:876). Validator helper is shared.
   - Test: `TestAppRouter_RelationMetaGate` covers PATCH-set, PATCH-unset,
     PATCH-writable-OK, POST-with-meta — four subtests.

8. **AC8 — SPA `DynamicForm` filters its field list against `_fields` (F1 + F19).**
   Unit test renders `DynamicForm` with a fixture entity carrying `_fields = {kind: {writable: false}, effort: {options: {l: false, xl: false}}}`
   (sparse per F13), `properties = {kind: "enhancement", effort: "m"}`, and a
   `data-entry.yaml` form config declaring `[title, kind, priority, effort, status]`.
   Asserts post-load:
   - `title` input rendered, enabled (no `_fields` entry → writable default)
   - `kind` input rendered, has `readonly`/`disabled` attribute
   - `priority` input **not in the DOM** (absent from `properties` AND
     `_fields`; F1 filter excludes)
   - `effort` rendered as `<select>` with no `<option value="l">` or
     `<option value="xl">`
   - `status` rendered, all options present
   Asserts during load (F19 flicker fix):
   - With `loading.value === true` and `entity === undefined`, the form
     does NOT render fields. (Either via a top-level `v-if="!loading"`
     guard, or via the filter returning `[]` when entity is undefined.
     Implementation picks the simpler option; test verifies the chosen
     behavior.)
   - Test: `frontend/src/components/forms/DynamicForm.test.ts` (NEW file,
     using vitest + @vue/test-utils). Two subtests: post-load filter
     behavior and during-load flicker prevention.

9. **AC9 — SPA `RelationCards` renders against `_relations`.** Unit test
   renders `RelationCards` with `_relations = {affects: {creatable: false, removable: true}, implements: {creatable: true, removable: false, fields: {note: {writable: false}}}}`. Asserts:
   - `affects` panel: `+ Add` button absent, per-link `x` button present
   - `implements` panel: `+ Add` button present, per-link `x` button absent
     on every link
   - `implements` meta-field `note` input has `disabled` attribute
   - Test: `frontend/src/components/forms/RelationCards.test.ts` (NEW or extend).

10. **AC10 — Manual end-to-end verification under `demo`.** With
    `RELA_AFFORDANCE_PROFILE=demo just dev` (F18 verified: `just dev` is
    `go run ./cmd/rela-server -project ... -port ...` — env passes through)
    and a ticket loaded:
    - `title` input editable; saves
    - `kind` input rendered read-only; auto-save no-op
    - `priority` field not in the form
    - `status` dropdown lacks `done`
    - `effort` dropdown lacks `l` and `xl`
    - `+ Add affects` button absent
    - `x` on `implements` links absent
    - Non-writable relation-meta input is disabled
    Evidence: screenshot or DOM snippet in implementation-checklist.

11. **AC11 — Create-mode is unaffected (F10).** `RELA_AFFORDANCE_PROFILE=demo`
    with a new-ticket form: all fields render normally (including `priority`),
    no enum options filtered. The collection-level GET emits `_actions.create`
    (unchanged from phase-1) and no `_fields` / `_relations` (the new wire keys
    are per-entity only in v1).
    - Test: `TestAppRouter_CollectionGet_DemoProfile_NoFieldVerdicts`.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Phase-1 affordances (`_actions`)** in `feat/acl-affordances` (PR #779,
  TKT-Y72A) is the direct architectural precedent. Key reuse:
  - `internal/dataentry/affordances.go` houses `translateVerb` +
    `computeActions` + `computeCollectionActions`. This ticket adds
    `computeFields` and `computeRelations` next to them, same shape.
  - `frontend/src/types/entity.ts` already documents the underscore-prefix
    convention and the "absent vs empty {}" closed-world semantics. The new
    `_fields` / `_relations` keys follow the same documentation pattern.
  - `affordances_contract_test.go` is the template for the bidirectional
    contract test (read says false ⇒ write returns 403). This ticket adds
    equivalent contract tests for field, option, relation-create/remove, and
    relation-meta paths.
  - `lint_test.go` enforces "no other site constructs `acl.WriteRequest{Op:`
    directly." This ticket doesn't add ACL ops — the stub gates writes at the
    handler layer, not the ACL layer — so the lint stays satisfied.

- **SPA infrastructure already exists for some rendering primitives** (F1
  correction):
  - `FieldRenderer.vue:12` accepts `readonly?: boolean`; line 142, 161, 191, 202
    plumb it through every widget. The new code passes this prop based on
    `entity._fields?.[name]?.writable === false`.
  - `FieldRenderer.vue:63` already has `isOptionDisabled(opt)` for transition
    rules. The new code extends this to also consult `entity._fields?.[name]?.options`.
  - `DynamicForm.vue:98-105` builds its `fields` list from
    `formConfig.sections.flatMap(s => s.fields)` — **NOT** from
    `entity.properties`. The plan's prior claim that "omitted = naturally hidden"
    was wrong. The new code adds an explicit filter step that drops fields whose
    property name is absent from both `entity._fields` AND `entity.properties`.
  - `RelationCards.vue` is the bigger SPA touch point — needs to read
    `_relations[type]` for `creatable`/`removable`/`fields` and disable the
    corresponding UI affordances. Per-link `x` button removal applies
    uniformly to every link of that type (F5).

- **Server-side PATCH path:**
  - `handleV1UpdateEntity` (api_v1.go:542) is the single entry point.
  - Properties are merged into `entity.Properties[k] = v` (line 614) without
    metamodel validation today — unknown fields are silently accepted. This
    ticket adds the affordance check at this point.
  - Relation-meta lives in `V1RelationsUpdate.Meta` / `MetaUnset`
    (`relations_v1_wire.go:58`) — single inspection point for AC7 (F6 resolved).
  - `properties_unset` already emits `unknown_property_unset_key` warnings for
    unknown keys (line 627) — informative precedent for the F3 friendly-drop
    warning shape.

- **Design docs:**
  - `.ignored/forms-acl-research.md` — survey of 11 systems; converged
    recommendation: payload-attached affordances (vs separate `/forms`
    endpoint), bool-map shape, closed-world contract.
  - `.ignored/action-affordances-design.md` — phase-1 design; locks in the
    `{verb: bool}` map shape and the "wire-vs-policy parity" invariant.
    This ticket inherits both.
  - `.ignored/condition-language-use-cases.md:386` — slot/outcome table that
    this ticket's wire shape implements. The predicate ticket eventually
    populates these from `*_when` expressions; this ticket populates them from
    a stub.

- **DEC-HWZHA (CLAUDE.md validation policy)** — soft-condition writes return
  `200 + warnings`, not 422. The "read-only field at same value → silent drop
  with warning" semantics (F3) is the direct application of this policy.

- **Libraries:** none. No new dependencies.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Server side

1. **Define `FieldVerdictResolver` interface in `internal/dataentry`:**

   ```go
   // FieldVerdictResolver decides per-entity affordances for fields,
   // enum options, and relation-meta fields. Wire shape is documented
   // in docs/data-entry/api-reference.md. v1 ships two implementations
   // (NopFieldVerdictResolver, DemoFieldVerdictResolver); the
   // predicate-engine ticket will replace them with a policy-driven
   // implementation.
   //
   // The resolver is consulted once per per-entity GET (to populate
   // _fields / _relations) and once per PATCH (to gate writes). It is
   // a constructor parameter on dataentry.App so tests pass it directly
   // without env-var manipulation.
   type FieldVerdictResolver interface {
       FieldVerdicts(ctx context.Context, e *entity.Entity) FieldVerdicts
       RelationVerdicts(ctx context.Context, e *entity.Entity) RelationVerdicts
   }

   type FieldVerdicts struct {
       // Writable: fieldName → writable. Absence = writable (sparse).
       Writable map[string]bool
       // Visible: fieldName → visible. Absence = visible (sparse). F20
       // unified the map convention (was `Hidden map[string]struct{}`).
       Visible  map[string]bool
       // Options: fieldName → optionValue → allowed. Absence of the field
       // OR absence of an option = allowed (sparse on both axes per F13).
       Options  map[string]map[string]bool
   }

   type RelationVerdicts struct {
       Types map[string]RelationVerdict          // relationType → verdict
   }

   type RelationVerdict struct {
       Creatable bool
       Removable bool
       Fields    map[string]bool                  // metaField → writable
   }
   ```

2. **Two implementations in `internal/dataentry/affordances_stub.go`:**
   - `NopFieldVerdictResolver` returns zero-value `FieldVerdicts` and
     `RelationVerdicts` for any entity. Wire emission still includes
     every metamodel-declared field as `{writable: true}` — see #6 below.
   - `DemoFieldVerdictResolver` applies the fixture from the "Demo profile
     fixture" table above when entity type is `ticket`; returns zero-value
     for all other types.

3. **Profile selection in `cmd/rela-server/main.go`** (per F7):

   ```go
   profile := os.Getenv("RELA_AFFORDANCE_PROFILE")
   var resolver dataentry.FieldVerdictResolver
   switch profile {
   case "", "none":
       resolver = dataentry.NopFieldVerdictResolver{}
   case "demo":
       resolver = dataentry.DemoFieldVerdictResolver{}
   default:
       slog.Warn("unknown RELA_AFFORDANCE_PROFILE, falling back to none", "value", profile)
       resolver = dataentry.NopFieldVerdictResolver{}
   }
   // Pass to dataentry.New(...)
   ```

   `dataentry.New` gains a `FieldVerdictResolver` parameter. Tests pass the
   resolver directly without setting env vars.

4. **`computeFields(ctx, e)`** and **`computeRelations(ctx, e)`** in
   `affordances.go` — SPARSE emission (F13):
   - Call `a.fieldResolver.FieldVerdicts(ctx, e)`.
   - Build the wire `_fields` map by emitting ONLY deviations from default:
     - For each `name` in `Writable` where `false`: emit `{writable: false}`.
     - For each `name` in `Options` with any `false` value: emit
       `{options: {<value>: false, ...}}` (only the false entries).
     - Merge writable + options entries when both deviate for the same field.
     - The `Visible` map controls property omission (see step 5) but does
       NOT emit an `_fields` entry for visible-default OR for hidden — hidden
       fields are absent everywhere (closed-world test in AC2).
   - Same sparseness for relations:
     - Emit `_relations[type]` only when at least one of `Creatable`,
       `Removable`, or any `Fields[meta]` deviates from default.
     - Within the entry, omit keys whose value is the default (`creatable:true`,
       `removable:true`).
   - Under `none` profile, both maps are emitted as `{}` (present but empty).

5. **`api_v1.go` per-entity GET handler:**
   - Call `computeFields`; serialize as `_fields`.
   - Call `computeRelations`; serialize as `_relations`.
   - Before serializing `properties`, delete keys where `FieldVerdicts.Visible[k]
     == false`.
   - Existing `_actions` emission is unchanged.

6. **Wire-vs-policy parity (revised per F12, F14, F16):**

   New helper `validateAffordances(req *updateReq, e *entity.Entity) error`:
   - Compute `FieldVerdicts` and `RelationVerdicts` for the target entity.
   - For each key in `req.Properties` AND each key in `req.PropertiesUnset`
     (F16: unset is a write):
     - If `Visible[k] == false`: error `field-affordance:hidden`.
     - If `Writable[k] == false`: error `field-affordance:read-only` (F12:
       strict 403 regardless of value).
     - If the field has `Options[k]` and the requested value isn't allowed:
       error `field-affordance:enum-filtered`.
     - If `k` is declared neither in the metamodel for `e.Type` NOR known
       by the resolver: error `field-affordance:hidden` (F8: byte-equivalent
       to genuinely-hidden; closes the side channel).
   - For relation-meta in `req.Relations.Modern[*].Refs[*].Meta` AND
     `MetaUnset`: same logic against `RelationVerdicts.Types[relType].Fields[metaKey]`.
   - For relation-edge add/remove implied by `req.Relations`: same logic
     against `RelationVerdicts.Types[relType].{Creatable, Removable}` (F14:
     unified PATCH path can add/remove relations).
   - First error wins; no partial application; return 403 immediately.

   Audit emits `denied-write` with `reason: "affordance:<rule>:<path>"` (F21).

7. **Three relation-write endpoints all gate (F14):** A shared validator
   `validateRelationOp(verdicts RelationVerdicts, relType string, op Op) error`
   lives in `affordances.go`. Each handler invokes it before delegating to
   `entityManager`:
   - `handleV1CreateRelation` (api_v1.go:863) — call validator with
     `op=Create` plus meta-field check via `validateRelationMeta`.
   - `handleV1DeleteRelation` (api_v1.go:960) — call validator with
     `op=Delete`.
   - `handleV1UpdateEntity` modern relations reconciler (api_v1.go:600 area)
     — walk the diff of current vs requested edge sets per type; validate
     Add and Delete operations. Meta-only updates (no edge change) hit
     `validateRelationMeta` only.
   All three return 403 with the same response shape.

8. **Per-relation-type uniform `removable` (F5):** DELETE of an `implements`
   link is gated by `_relations.implements.removable` for any link of that
   type. The 403 response shape is `{rule: "relation-affordance:not-removable",
   type: "implements"}` — no link identifier in the rule.

### SPA side

1. **`frontend/src/types/entity.ts`**: extend `Entity` interface:

   ```ts
   _fields?: Record<string, { writable: boolean; options?: Record<string, boolean> }>
   _relations?: Record<string, {
     creatable: boolean
     removable: boolean
     fields?: Record<string, { writable: boolean }>
   }>
   ```

2. **`DynamicForm.vue` — new active filter (F1 critical fix):** The `fields`
   computed becomes:

   ```ts
   const fields = computed((): FormFieldOrRelation[] => {
     const all = /* current config-driven list */
     // In create mode (no entityId), no entity available; render all.
     if (!props.entityId) return all
     const entity = entitiesStore.get(formConfig.value.entity, props.entityId)
     if (!entity?._fields && !entity?.properties) return all  // pre-rollout server fallback
     return all.filter(f => {
       if (!f.property) return true  // relations / non-property fields untouched
       return f.property in (entity._fields || {}) || f.property in (entity.properties || {})
     })
   })
   ```

3. **`DynamicForm.vue` per-field readonly:** Computes `field.readonly` from
   `entity._fields?.[name]?.writable === false`, plumbed into existing
   `:readonly="field.readonly"` props (lines 854, 889).

4. **`FieldRenderer.vue`**: extend `isOptionDisabled(opt)` to also consult
   `props.field.optionVerdicts?.[opt] === false`. Pass `optionVerdicts` from
   `DynamicForm.vue`.

5. **`RelationCards.vue`**: read `entity._relations?.[relType]` and:
   - Wrap the `+ Add` button in `v-if="!verdict || verdict.creatable !== false"`
   - Wrap the per-link `x` button similarly with `removable`
   - Pass `:disabled` to inline meta-field inputs based on
     `verdict?.fields?.[propName]?.writable === false`

6. **No new stores, no new API client functions.** The new wire keys ride on
   `Entity` and existing CRUD calls.

**Files to modify:**

| File | Change |
|---|---|
| `internal/dataentry/affordances.go` | Add `computeFields`, `computeRelations`, `applyFieldVerdicts`, types |
| `internal/dataentry/affordances_stub.go` (NEW) | `NopFieldVerdictResolver`, `DemoFieldVerdictResolver` |
| `internal/dataentry/app.go` | Add `fieldResolver FieldVerdictResolver` field; constructor param |
| `cmd/rela-server/main.go` | Parse `RELA_AFFORDANCE_PROFILE`, pass resolver to `dataentry.NewApp` (line 119) |
| `cmd/rela-desktop/main.go` | Same env-var parse + pass to `dataentry.NewApp` (line 154; F17-verified the binary calls the same constructor) |
| `internal/dataentry/api_v1.go` | Per-entity GET emits new keys; PATCH consults verdicts |
| `internal/dataentry/api_v1_test.go` | Existing tests assert new wire keys present-and-baseline under `none` |
| `internal/dataentry/affordances_test.go` | Unit tests for `computeFields` / `computeRelations` / `applyFieldVerdicts` |
| `internal/dataentry/affordances_contract_test.go` | Bidirectional contract tests for AC3–AC7 |
| `frontend/src/types/entity.ts` | Add `_fields`, `_relations` keys with docstring |
| `frontend/src/components/forms/DynamicForm.vue` | F1 filter; readonly plumbing; option-verdict plumbing |
| `frontend/src/components/forms/FieldRenderer.vue` | Extend `isOptionDisabled` to consult option verdicts |
| `frontend/src/components/forms/RelationCards.vue` | Add/remove/meta-field affordances |
| `frontend/src/components/forms/DynamicForm.test.ts` (NEW) | AC8 assertions; vitest infrastructure if not present |
| `frontend/src/components/forms/RelationCards.test.ts` (NEW or extend) | AC9 assertions |
| `docs/data-entry/api-reference.md` | Document `_fields` / `_relations` wire shape; F11: drop docs/security.md mention from plan |

**Alternatives considered:**

- **`acl.yaml` schema extension with `writable_for: [roles]`.** Rejected:
  no real users yet, deletion cost outweighs dogfooding value, and predicate
  ticket replaces source anyway.
- **Per-link affordances.** Rejected for v1: requires state-dependent gates
  (predicate territory).
- **Mask hidden fields** (`{value: null, _fields[name].visible: false}`).
  Rejected: omit is strictly more secure (no key leak); the F1 fix adds the
  needed SPA filter to make omission actually hide.
- **HTTP-header profile override.** Rejected: env var is simpler, lower risk
  of being forgotten in prod.
- **Strict 403 for read-only field at same value (original F3 design).**
  Rejected per F3 review: hostile to mixed PATCHes from stale clients. Replaced
  with silent-drop-with-warning per DEC-HWZHA.
- **Stack PR on top of #779 (`feat/acl-affordances`).** Rejected per F2:
  reintroduces the PR-gates-PR problem. Branch off develop directly.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation |
|---|---|---|
| `RELA_AFFORDANCE_PROFILE` env var | server startup | Allowlist: `none`, `demo`. Unknown → warn, default to `none`. Never panic. |
| Field names in PATCH body | HTTP client | New: cross-check against resolver's `Writable` allowlist + metamodel-declared fields. Unknown fields (neither declared nor in resolver) → 403, same shape as hidden (F8). |
| Enum option values in PATCH | HTTP client | Cross-check against `FieldVerdicts.Options` allowlist for that field. |
| Relation type / meta-field names in PATCH | HTTP client | Cross-check against `RelationVerdicts.Types` allowlist. |

**Security-Sensitive Operations:**

- **Wire-vs-policy parity is the security-critical invariant.** Stale SPA
  clients cannot bypass affordance gates: every write path consults the same
  resolver. Bidirectional contract tests (AC3–AC7) lock this down.
- **Hidden field omission AND F8 side-channel closure.** Hidden-field PATCH
  and truly-unknown-field PATCH produce structurally identical 403 responses.
  No inference channel: an attacker cannot distinguish "hidden from me" from
  "doesn't exist." Test: `TestAppRouter_PatchUnknownField_Forbidden` asserts
  the response is byte-equivalent to the hidden case.
- **No information leak in rejection messages.** 403 reasons use stable rule
  identifiers (`field-affordance:read-only`), never role names or user IDs.
- **DEC-HWZHA friendly-drop semantics** for read-only same-value PATCHes are
  a UX accommodation, not a security relaxation: the write doesn't apply, the
  warning is logged, the audit log records the dropped field. Tested at AC4.
- **No new principal handling, no new audit ops, no new crypto.** The
  `denied-write` audit op already covers the new 403 cases; the rule
  identifier rides in the existing `reason` field.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
|---|---|
| AC1 | `TestAppRouter_PerEntityGet_NoneProfile` — wire shape under default |
| AC2 | `TestAppRouter_PerEntityGet_Demo` — wire shape under demo profile |
| AC3 | `TestAppRouter_PatchHiddenField_Forbidden` + `TestAppRouter_PatchUnknownField_Forbidden` — F8 parity |
| AC4 | `TestAppRouter_PatchReadOnlyField` — three subtests (same/different/mixed) |
| AC5 | `TestAppRouter_PatchFilteredOption_Forbidden` |
| AC6 | `TestAppRouter_RelationGates_Forbidden` — POST/DELETE |
| AC7 | `TestAppRouter_RelationMetaGate` — same/different/writable subtests |
| AC8 | `DynamicForm.test.ts` — F1 filter; readonly; option filtering |
| AC9 | `RelationCards.test.ts` — add/remove/meta-field |
| AC10 | Manual e2e |
| AC11 | `TestAppRouter_CollectionGet_DemoProfile_NoFieldVerdicts` |

**Edge Cases:**

- **Empty `_fields` map vs absent.** Under `none`, `_fields` is present and
  populated with all-writable defaults. Under a pre-rollout server (no key
  at all), SPA's filter falls back to rendering everything (the
  `entity._fields ?? {}` path).
- **Anonymous principal.** Resolver is called the same way; the `none`
  profile returns zero verdicts regardless of principal.
- **Field declared in metamodel but not present on entity instance** (sparse
  property). Wire `_fields` lists it as `{writable: true}` (default); SPA
  renders normally. Hidden status is independent of value presence.
- **Form-config field not in metamodel.** Should not occur in valid configs;
  if it does, the SPA filter's "in `_fields` OR in `properties`" check leaves
  it rendering (no entry in either, so it falls through to the unfiltered
  state). Documented but not tested — this is a config-error case the
  metamodel validator should catch separately.
- **Create-mode (`!entityId`).** SPA filter short-circuits, renders all
  config-declared fields. Tested at AC11.
- **Pre-rollout server (no `_fields` in response).** SPA filter short-circuits
  via the `if (!entity?._fields && !entity?.properties)` guard, renders all.
- **Empty `properties` on entity** (degenerate, shouldn't happen). Same
  fallback as pre-rollout.
- **Relation type with no entry in `_relations`.** All operations allowed
  (default).
- **PATCH includes hidden field with value matching nothing** (the field has
  no value because it's hidden). Still 403 — SPA shouldn't have known about
  the field; sending it is an attack surface. Same shape as F8.
- **Race: server reloads profile while a request is in flight.** Not relevant
  — resolver chosen at startup, never reassigned.
- **Concurrent writes targeting filtered options.** Existing entity-level
  `writeMu` (api_v1.go:544) serializes; affordance check is purely local to
  the request.

**Negative Tests:**

- Unknown `RELA_AFFORDANCE_PROFILE` value → warning logged, falls back to
  `none`. Test: `TestNew_UnknownProfile_FallsBackToNone` (constructor-level,
  no env var).
- Resolver returns nil maps → handler doesn't panic, emits baseline shape.
  Test: `TestComputeFields_NilMaps_EmitsBaselineShape`.
- PATCH with hidden + writable fields in same body → 403 wins; no partial
  write. Test: `TestAppRouter_PatchMixedHiddenAndWritable_Forbidden`.
- PATCH with read-only-same-value + truly-writable fields → 200, writable
  field applied, warning emitted. Test in AC4 subtest 3.

**Integration test approach:**

- AC3–AC7 use the existing `internal/dataentry/affordances_contract_test.go`
  fixture style: bring up an `App` with the `DemoFieldVerdictResolver`,
  exercise the HTTP surface end-to-end, assert both the GET shape and the
  corresponding mutation outcome. No mocking of resolver — uses the real
  `DemoFieldVerdictResolver`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| **#779 doesn't merge before this lands.** Per F2: branch off develop directly. If #779 still open at merge time, this PR reshapes `V1Entity.Actions` from `*V1Actions` to `map[string]bool` (the phase-1 work, ~80 LOC). Conflict resolution at merge time is mechanical. | Budget includes this contingency. |
| **F1 SPA filter may interact unexpectedly with `formConfig` sections** (some forms group fields by section). The filter runs across the flattened list; a section may end up empty. | AC8 test fixture exercises a section-organized form. If sections become empty, hide the section header too (small render guard). |
| **Vitest test infrastructure may not exist** for `DynamicForm.vue` and `RelationCards.vue`. | AC8 / AC9 explicitly call out NEW test files. Effort estimate reflects this. |
| **Metamodel-declared enum values not currently surfaced in `_fields` baseline.** Computing `options: {all: true}` requires walking the entity-type's enum definitions. | Use existing `metamodel.Properties` schema from `internal/metamodel`; no new metadata. |
| **Write-vs-read race in PATCH parity check.** Resolver consulted on the request entity; if another writer changes state mid-PATCH, resolved verdicts may be stale. | Acceptable: `none` and `demo` are state-independent. Predicate ticket inherits this race-window and addresses via snapshot-at-handler-top (CLAUDE.md pattern). |
| **Demo profile's relation-meta target depends on what `tickets/metamodel.yaml` actually exposes.** | Implementation-checklist task: audit metamodel, pick a relation that has any writable meta property. If none exist, the relation-meta fixture targets an entity type that does (e.g. a test-only metamodel fixture). |
| **Audit log will see more `denied-write` entries under `demo`.** | Acceptable — demo is dev-only. Documented in api-reference. |

**Effort:** **l** (large) — server: ~400 LOC + tests (more than original
estimate due to F8 unknown-field handling and F3 friendly-drop semantics);
SPA: ~150 LOC + tests; plus design-review and code-review rounds. Realistic
2–3 day implementation if attention isn't broken up.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact (F11 cleaned up):**

- [x] **API docs** — `docs/data-entry/api-reference.md` documents `_fields` and
  `_relations` wire shape next to the existing `_actions` documentation.
- [x] ~~User guide / reference~~ (N/A: the stub source is dev-only; users
  won't see field-level affordances meaningfully until the predicate ticket
  lands.)
- [x] ~~`docs/security.md` mention~~ (N/A: deferred to predicate ticket;
  this ticket has no security-policy semantics to document for end users.)
- [x] ~~CLI help text~~ (N/A: env var documented in api-reference only.)
- [x] ~~CLAUDE.md~~ (N/A: wire-shape detail is below CLAUDE.md abstraction.)
- [x] ~~README.md~~ (N/A.)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings (round 1, addressed in this revision):**

- F1 [critical] — SPA filter against `_fields`/`properties` added; `DynamicForm.vue`
  `fields` computed gains active filter step. AC8 rewritten to test this.
- F2 [significant] — Branching decision: off develop, accept potential
  `V1Entity.Actions` reshape contingency.
- F3 [significant] — Read-only same-value PATCH → silent drop with warning,
  not 403. Aligns with DEC-HWZHA. AC4 rewritten.
- F4 [significant] — Demo fixture rebuilt against actual `ticket` properties
  (`title, kind, priority, effort, status`); table in scope section pins
  exact verdicts.
- F5 [significant] — `removable` confirmed per-relation-type uniform; AC6
  pins error shape (no link identifier).
- F6 [significant] — Relation-meta inspection point identified
  (`V1RelationsUpdate.Meta` / `MetaUnset`); single chokepoint, no refactor.
- F7 [minor] — Resolver becomes constructor parameter; env var parsed in
  `cmd/rela-server/main.go`. Tests pass resolver directly.
- F8 [minor] — Hidden + unknown fields produce structurally identical 403;
  `TestAppRouter_PatchUnknownField_Forbidden` asserts equivalence.
- F9 [minor] — Profile renamed `triager-demo` → `demo`.
- F10 [minor] — Create-mode unrestricted in v1; AC11 added.
- F11 [nit] — Documentation Planning cleaned up; ambiguous strikethroughs
  removed.

**Design Review Findings (round 2, addressed in this revision):**

- F12 [significant] — Dropped friendly-drop semantics for read-only fields.
  `useAutoSave` already suppresses no-op PATCHes client-side (`lastSeenServer`).
  The matching-value branch was dead code. AC4 collapsed to strict 403.
- F13 [significant] — Wire shape is SPARSE: `_fields` and `_relations` carry
  only deviations from default. Under `none`, both emit as `{}`. SPA uses
  `entity.properties` (not `_fields`) as visibility signal. AC1/AC2 rewritten;
  `FieldVerdicts` semantics clarified.
- F14 [significant] — Three relation-write chokepoints, not one:
  `handleV1CreateRelation` (POST), `handleV1DeleteRelation` (DELETE), and the
  modern reconciler inside `handleV1UpdateEntity` (PATCH). Shared validator
  `validateRelationOp` invoked at all three. AC6 expanded.
- F15 [significant] — List-typed enum option-filter (e.g. `tags`) deferred.
  Wire shape supports it; PATCH validator only walks scalar enums in v1.
  Documented in OUT.
- F16 [minor] — `properties_unset` and `MetaUnset` now treated as writes for
  hidden/read-only checks. AC3, AC4, AC7 cover the unset cases.
- F17 [minor] — `cmd/rela-desktop/main.go` confirmed to call `dataentry.NewApp`
  at line 154; env-var parse + resolver pass-through added there too.
- F18 [minor] — `just dev` confirmed to be plain `go run`, env passes through.
  Manual e2e command documented in AC10.
- F19 [minor] — SPA flicker prevention during load. `DynamicForm` either
  blocks rendering until `loading.value === false` OR the filter returns `[]`
  on undefined entity. Implementation chooses; AC8 covers both load states.
- F20 [nit] — `FieldVerdicts` uses uniform `map[string]bool` convention
  (renamed `Hidden map[string]struct{}` → `Visible map[string]bool` with
  inverted polarity).
- F21 [nit] — Audit `reason` prefix discipline: `affordance:<rule>:<path>`
  for stub denials vs `acl:...` for real ACL denials. No new audit op for v1;
  predicate ticket re-evaluates.
