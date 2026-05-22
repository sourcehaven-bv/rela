---
id: PLAN-XJ60
type: planning-checklist
title: 'Planning: Action affordances phase 2: frontend consumption + AWM6L payoff'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope (phase 2).** Vue components consult `entity._actions[verb]` /
`collection._actions[verb]` at every phase-1 verb-call-site (`create`, `update`,
`delete`). Boolean false → omit the control; anything else (true, undefined,
absent) → render. Add a dev-mode console warning when an authenticated session
receives a response without `_actions` from a whitelisted endpoint. E2E asserts
the AWM6L payoff (with documented exceptions): no entity-CRUD write controls
render in read-only mode.

**Out of scope** — explicitly visible-but-not-gated in phase 2 (will 404/403 on
click):

- `transition:*` / `relation:*` verbs (gated on TKT-XZEY / ACL v0.5).
- Lua action / command buttons (`runCommand`, `executeAction`) — out
of phase-1 verb vocabulary; future `action:<id>` verb in a later ticket. The
`--read-only` server flag still 403s these at the endpoint.
- Settings / theme / git / scheduler write paths — out of phase-1 verb
vocabulary; `--read-only` flag covers them at the server level.
- `useACL()` composable or any centralised policy resolver — the SPA
reads booleans directly from the resource payload (TKT-AWM6L's rejected approach
stays rejected).
- Per-property affordances.
- DynamicForm form-wide read-only mode. **Per user direction (see
C2 below):** non-updatable entities don't reach the form. No sub-widget readonly
props, no `<DynamicForm readonly>` toggle.

**Inventory of call sites** (re-greped after audit; ~14 entity-CRUD sites in
scope, ~10+ deferred):

| Site | File:line | Verb | Phase 2 status |
|---|---|---|---|
| List "+ New" button | `EntityList.vue:560` | `create` | gate |
| List row delete (compact) | `EntityList.vue:657` | `delete` | gate |
| List row delete (tile) | `EntityList.vue:794` | `delete` | gate |
| List Del/Backspace handler | `EntityList.vue:138` | `delete` | gate |
| List bulk action bar | `EntityList.vue:51-58, 79` | `update` (per-action) | gate |
| List ad-hoc property apply | `useListActions.ts:76` | `update` | gate |
| Kanban "+ New" | `KanbanView.vue:322` | `create` | gate |
| Kanban drag-drop status | `KanbanView.vue:266` | `update` | gate |
| EntityDetail Del key | `EntityDetail.vue:166` | `delete` | gate |
| EntityDetail Edit button (desktop) | `EntityDetail.vue:382` | `update` | gate |
| EntityDetail Edit button (mobile) | `EntityDetail.vue:396` | `update` | gate |
| EntityDetail inline-edit in related cards (×6) | `EntityDetail.vue:530-693` | `update` | gate |
| DynamicForm submit / auto-save | `DynamicForm.vue:413` | `update` | gate (transitive via Edit-button gating + route guard) |
| Form route guard | `/edit/:type/:id` handler | `update` | NEW — render "not editable" message when `_actions.update === false` |
| InlineCreateModal from list "+ New" | transitive | `create` | gated via "+ New" |
| InlineCreateModal from RelationPicker | `RelationPicker.vue:353` | `create` (target type) | gate this direct entry too |
| EntityDetail command buttons (Lua) | `EntityDetail.vue:371-378, 415-422` | (deferred) | not gated; 403 at server in read-only |
| RelationCards add/remove | `RelationCards.vue:367, 487` | `relation:*` deferred | not gated |
| RelationPicker remove | `RelationPicker.vue:307` | `relation:*` deferred | not gated |
| Settings/theme/git writes | various | (deferred) | not gated |

**Acceptance Criteria** (revised after audit):

1. **AC1 — list `+ New` button.** Renders iff
`listResponse._actions?.create !== false`. Component unit test against three
fixtures (`create: true`, `create: false`, absent).
2. **AC2 — list row delete.** Both compact and tile layouts gate
delete buttons on `entity._actions?.delete !== false`. Unit test with
mixed-permission fixture rows.
3. **AC3 — list Del-key.** Pressing Del/Backspace on a row whose
`_actions.delete === false` surfaces the existing `uiStore` toast "Delete not
permitted" — no confirm modal, no API call. Unit test.
4. **AC4 — Kanban gating.** `+ New` button gated on collection
`_actions.create`. Drag-drop gated via `:draggable` binding *and* `@drop`
early-return on `_actions.update`. Unit tests for both the drag-start and the
drop paths.
5. **AC5 — EntityDetail Del-key.** Gated on `_actions.delete`; same
toast feedback as AC3.
6. **AC6 — EntityDetail Edit button.** Both desktop and mobile Edit
buttons render iff `entity._actions?.update !== false`. Component unit test.
7. **AC7 — Form route guard.** Direct navigation to `/edit/:type/:id`
when the loaded entity's `_actions.update === false` renders an inline "This
entity is not editable" message with a back-to-detail link. Unit test + E2E.
8. **AC8 — Bulk action bar.** Action bar hides when no selected row
permits the action (i.e. every selected entity has `_actions[verb] === false`
for the bar's verb). Unit test.
9. **AC9 — Dev-mode warning fires + dedupes.** When `import.meta.env.DEV
=== true` and a whitelisted API response (`listEntities`, `getEntity`,
`createEntity`, `updateEntity`) omits `_actions`, `console.warn` fires exactly
once per request-path (dedup via module-level `Set<string>`; reset on HMR).
Production build (DEV=false) emits no warnings. Unit test for both the
fires-once and the dedup-on-repeat cases.
10. **AC10 — AWM6L E2E.** `e2e/tests/read-only-mode.spec.ts` (new
file): boot `rela-server --read-only`, load `/list/ticket`, assert no
entity-CRUD controls render (no "+ New", no row delete buttons, no Edit button
on entity detail). Lua / settings / git buttons may still render — that's
documented as deferred.
11. **AC11 — Additive vocabulary (frontend-only).** Inject a fixture
response with extra `{noop: true}` key; assert components render without console
error and the unknown key doesn't surface in any UI.
12. **AC12 — Backend per-type test.** Add a `computeActions` unit test
in `internal/dataentry/affordances_test.go` calling against a `Declarative`
policy with mixed type grants; assert verb maps differ across types. (Per-row
within-type variance is deferred to TKT-XZEY since ACL v0 has no row context.)

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

Phase-1 research base applies (see `.ignored/action-affordances-research.md`).
Additional check for phase 2: Vue ecosystem authorization libraries
(`@casl/vue`, `vue-router-permission-guard`) were considered and rejected — they
expect a frontend ACL model, which contradicts the design's "SPA reads
server-supplied booleans, doesn't compute" rule.

**Audit synthesis:** see `.ignored/affordances-phase2-audit-synthesis.md` for
the full audit findings (go-architect + cranky-code-reviewer) and the
resolutions taken.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **No new composable.** Each component reads
`entity._actions?.[verb]` directly. Two-line contract: `false → hide`, anything
else → render.
2. **Helper function only for the dev warning.** A single
`warnIfMissingActions(response, requestPath)` utility in
`frontend/src/utils/affordancesWarning.ts`. Called from a whitelist of API
client functions (`listEntities`, `getEntity`, `createEntity`, `updateEntity` —
**not** `searchEntities`, `analyze*`, SSE handlers, or any non-entity endpoint).
Dedup via module-level `Set<string>` keyed by request path; reset on HMR.
3. **Form route guard for non-updatable entities.** The
`/edit/:type/:id` route handler (`Form.vue` or equivalent) checks
`entity._actions?.update` after the GET; if `false`, renders an inline "not
editable" message + back link in place of `<DynamicForm>`. No form-wide readonly
mode; no sub-widget readonly props. The Edit button on EntityDetail is also
gated, so users reaching the form must have either bookmarked a URL or pasted a
link.
4. **Defensive fallback (absent → render).** The data-entry server
always emits `_actions`. Absent only happens for non-data-entry callers
(defensive). Show-all + server-side re-authz is the right trade-off. **Empty
`{}` and absent treated identically by the consumer**: `entity._actions?.[verb]
!== false` is the check. Phase-1's three-state distinction collapses to
two-state in phase 2; phase-1 design doc will be amended to admit this.
5. **Kanban drag-drop:** gate via `:draggable` binding (HTML5 string
attribute, so `:draggable="entity._actions?.update !== false ? 'true' :
'false'"`). Defence in depth: `@drop` handler early-returns when target
`_actions.update === false`.
6. **Bulk action bar gating:** the bar checks if *any* selected row
has `_actions[verb] !== false` for any bar action. If every selected row denies
every action, hide the bar. Same component already iterates `selectedIds`.

**Alternatives considered (and rejected):**

- **`<ActionGuard verb="...">` wrapper component** — over-engineering
for <20 sites; direct `v-if` is more honest. Revisit if phase 3 adds many
parameterised verbs (transitions etc.).
- **ESLint rule forbidding ungated write handlers** — same reasoning;
inventory is small enough for code review to catch drift.
- **Frontend `useACL()` mirror** — TKT-AWM6L wont-fix; stays killed.
- **Form-wide DynamicForm readonly mode** — per user direction, no
readonly form mode. Non-updatable entities don't reach the form.
- **Strict empty-map check** (`Object.keys(_actions).length === 0` →
hide all) — collapses the three-state distinction. Accepted; phase-1 design doc
to be amended.
- **`=== true` instead of `!== false`** — brittle (any verb omission
hides the button). The defensive `!== false` is the right default.

**Files to modify:**

| File | Change |
|---|---|
| `frontend/src/utils/affordancesWarning.ts` (NEW) | `warnIfMissingActions` helper + dedup Set + HMR reset. |
| `frontend/src/api/entities.ts` | Wire warning into `listEntities`, `getEntity`, `createEntity`, `updateEntity`. **Not** into `searchEntities`, `getEntityRelations`, etc. |
| `frontend/src/components/lists/EntityList.vue` | Gate "+ New" button on collection `_actions.create`. Gate per-row delete (both layouts) on `_actions.delete`. Gate Del-key handler with toast feedback. Gate bulk action bar visibility. |
| `frontend/src/composables/useListActions.ts` | Gate ad-hoc apply path on per-row `_actions.update`. |
| `frontend/src/views/KanbanView.vue` | Gate "+ New" on collection `_actions.create`. Gate `:draggable` binding + `@drop` handler on item `_actions.update`. |
| `frontend/src/components/entity/EntityDetail.vue` | Gate desktop + mobile Edit buttons on `_actions.update`. Gate Del-key handler on `_actions.delete` (same toast as list). Gate the 6 inline-edit buttons in related-entity cards. |
| `frontend/src/views/Form.vue` (or wherever `/edit/:type/:id` resolves) | After GET, check `_actions.update`; if false, render "not editable" message with back link instead of `<DynamicForm>`. |
| `frontend/src/components/forms/RelationPicker.vue` | Gate the "+ Create new" affordance (`openCreateModal`) on target-type collection `_actions.create`. |
| `internal/dataentry/affordances_test.go` | AC12 — `computeActions` unit test with mixed-type Declarative grants. |
| `e2e/tests/read-only-mode.spec.ts` (NEW) | AC10 — boot `rela-server --read-only`, walk SPA, assert no entity-CRUD buttons render. Lua / settings / git documented as expected-visible. |
| `docs/data-entry/api-reference.md` | Update §"How the SPA consumes `_actions`" (new section, not "the cardinal rule") to note that the SPA now actively gates write controls on the map. |
| `docs/security.md` | Update §"What the ACL covers in v0": SPA hides entity-CRUD controls in read-only mode; Lua/settings/git still render. |
| `.ignored/action-affordances-design.md` | Amend §"Anonymous principal handling" to admit the consumer-side two-state collapse (empty {} and absent both render). |
| `CLAUDE.md` | Add §"Action affordances (`_actions`)" subsection rule: "When adding a new entity-CRUD button in a Vue component, gate on `entity._actions?.[verb] !== false`. New verbs require backend `translateVerb` + `perItemVerbs`/`perCollectionVerbs` entries." |

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **`_actions` map from server.** Read-only consumption. No client-
supplied data influences the verdict.

**Security-Sensitive Operations:**

- **The cardinal rule is unchanged.** The server re-authorizes every
write. AC10 asserts the read-only-mode UX; phase-1's contract test
(`TestAffordances_BidirectionalContract`) asserts the enforcement invariant.
Both must remain green.
- **Defensive fallback is safe.** Absent → render. Server still 403s
on click.
- **No client-side ACL evaluation.** SPA reads booleans; no
computation, prediction, or merge. This is the duplication-trap prevention from
TKT-AWM6L's wont-fix.
- **Form route guard is UX, not security.** The "not editable"
message blocks the form render; an attacker can still PATCH directly (the server
403s). The guard exists to give the user a clear message instead of a broken
form.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test scope | Location |
|---|---|---|
| AC1 | `+ New` button visibility | `EntityList.test.ts` |
| AC2 | Row delete buttons (mixed-permission) | `EntityList.test.ts` |
| AC3 | Del-key toast feedback | `EntityList.test.ts` |
| AC4 | Kanban drag-start + drop both gated | `KanbanView.test.ts` (new) |
| AC5 | EntityDetail Del-key gating | `EntityDetail.test.ts` |
| AC6 | EntityDetail Edit buttons gated | `EntityDetail.test.ts` |
| AC7 | Form route renders "not editable" | `Form.test.ts` + `e2e/tests/read-only-mode.spec.ts` |
| AC8 | Bulk action bar gating | `EntityList.test.ts` |
| AC9 | Dev-mode warning fires + dedupes | `affordancesWarning.test.ts` (new) |
| AC10 | Read-only UI has no entity-CRUD controls | `e2e/tests/read-only-mode.spec.ts` (new) |
| AC11 | Synthetic verb fixtures don't error | `EntityList.test.ts` (extra-key fixture) |
| AC12 | `computeActions` mixed-type Declarative | `internal/dataentry/affordances_test.go` |

**Edge Cases:**

- **`_actions` absent on authenticated response.** Defensive render +
dev-mode warning fires.
- **Empty `{}` map.** Same as absent (renders all). Phase-1 design's
three-state distinction collapses to two-state consumer-side.
- **Mixed-permission list rows.** Each row consults per-row map.
- **Verb true at fetch, denied on click.** Server 403s; SPA shows
toast via existing error path.
- **Form direct-URL navigation when not editable.** Route guard
shows message; user clicks back link → returns to detail.
- **Kanban drag-drop with `update: false`.** `:draggable` set to
`'false'` (string); `@drop` early-returns; entity card stays put.
- **Bulk action bar with single row that denies all.** Bar still
visible because *some* other selected row might permit. Bar hides only when
*every* selected row denies *every* bar action.
- **SSE refresh during a session.** `entity._actions` refreshes on
the next cache hit (1-min TTL or invalidation). Stale verdicts bounded by TTL;
server enforces correctness on click.
- **Dev-mode warning during HMR.** Dedup Set resets via
`import.meta.hot.dispose` so re-warns aren't suppressed by stale state from a
prior module instance.

**Negative Tests:**

- **API client returns 4xx with `_actions` field.** Field ignored;
existing error path renders toast.
- **`updateEntity` succeeds, optimistic cache update, then policy
reload tightens.** Acknowledged as snapshot skew; out-of-scope fix.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| Missed call site discovered by E2E. | Audited inventory after grep; ~14 sites in phase 2. E2E (AC10) walks read-only mode; finding a missed site is the failure signal. |
| Dev-mode warning floods test output. | Component test fixtures inject `_actions` explicitly (which AC tests do anyway). The dedup Set bounds production cases. |
| Drag-drop UX glitch. | Gate `:draggable` at source (no drag starts) + `@drop` early-return (defence). AC4 tests both paths. |
| Bulk action bar wrong gating (hide too aggressively). | The "hides when no selected row permits any action" check is per-bar-action, not per-row. Unit test with mixed-permission selections. |
| Form route guard breaks edit-flow E2E. | The guard checks `_actions.update`; existing E2E uses NopACL which grants update=true. No regression for default path. AC7 adds positive + negative cases. |
| Optimistic cache update on a write that later 403s. | Out-of-scope fix; document. Existing rollback in `entitiesStore` (if any) inherits. |
| SSE policy-reload staleness window (~1 min TTL). | Documented in design doc Open Q5; phase 2 doesn't address. Stale verdicts → server enforces. |

**Effort:** m. Per-site changes are mostly one-line `v-if` additions; the larger
items are the Form route guard, the bulk-action-bar gating, and the dev-mode
warning helper.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] **`docs/data-entry/api-reference.md`** — new §"How the SPA
consumes `_actions`" section explaining the gating behaviour. **Do not modify
§"The cardinal rule"** (still applies as-is — server enforces).
- [x] **`docs/security.md`** — update §"What the ACL covers in v0":
SPA hides entity-CRUD controls in read-only mode; deferred buttons
(Lua/settings/git) still render and 403 at server.
- [x] **`.ignored/action-affordances-design.md`** — amend §"Anonymous
principal handling" to document the consumer-side two-state collapse.
- [x] **CLAUDE.md** — new "Action affordances" subsection rule
(under §Authorization).
- [x] ~~User guide / reference docs~~ (N/A: SPA UX is itself the
user-facing surface).
- [x] ~~CLI help text~~ (N/A).
- [x] ~~README.md~~ (N/A).

## Design Review

- [x] Run `/design-review` before starting implementation —
**go-architect + cranky audits round 1, user crit (synthesis approved)**
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- **go-architect (round 1)** — 1 critical (AC8 unimplementable
under ACL v0), 2 significant (test placement, AC9 synthetic-verb approach), 4
minor + 1 confirmation. All addressed.
- **cranky-code-reviewer (round 1)** — 4 critical (inventory off by
3x, DynamicForm readonly nonexistent, AC8 unimplementable, AC9 production
mutation), 8 significant, 7 minor, 3 nit. All criticals + significants addressed
in synthesis + this plan.
- **User crit on synthesis** — approved (round 1, no comments).
Inline direction: "no readonly forms; gate the Edit button + show error on
direct URL navigation."

Full synthesis at `.ignored/affordances-phase2-audit-synthesis.md`.
