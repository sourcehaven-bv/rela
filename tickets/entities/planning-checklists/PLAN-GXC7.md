---
id: PLAN-GXC7
type: planning-checklist
title: 'Planning: Response-level action affordances: backend declares per-resource verbs to drive UI'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope.** Make the backend the single source of truth for which write verbs
a principal can apply to a specific resource. The data-entry SPA renders write
affordances iff the backend declared the verb in the response's `_actions` map.
Phase 1 vocabulary is exactly the verbs ACL v0 supports today: `{create, update,
delete, rename}` — `create` per-collection, the others per-item. Drives off the
existing `acl.AuthorizeWrite` call.

**Out of scope.**

- Authorization is **not** moved to the wire. The server re-authorizes every
write. The map is a UI hint.
- **`transition:*` and `relation:*` verbs.** Deferred to TKT-Y72A-followup,
gated on ACL v0.5. (Audit finding: ACL v0's `Op` enum doesn't carry the required
arguments.)
- Per-property affordances.
- Multi-action query plans.
- A separate `/meta/actions` discovery endpoint. Crit round 2 surfaced that
collection-level `_actions.create` covers menu population for free. No separate
endpoint in phase 1.
- Parallel-emit / `_actions_v2`. Crit round 2: "legacy stuff is not needed."
Reshape `_actions` in place; backend + SPA migrate in the same PR.
- A new frontend `useACL()` composable.
- i18n of action labels.
- SSE-driven affordance updates.
- MCP / Lua / scheduler write paths' affordance integration. Scope of the
invariant is data-entry HTTP API only.

**Acceptance Criteria** (mirrored from design doc §AC1–AC10):

1. **AC1 — read-only verdict.** GET an entity as a read-only principal returns
`_actions: {update: false, delete: false, rename: false}`. Handler test.
2. **AC2 — nop verdict.** GET as a `NopACL` principal returns `_actions` with
all three verbs `true`. Handler test.
3. **AC3 — bidirectional contract test.** For a fixed (principal, entity)
tuple, every verb V where `_actions[V] == true` → write returns 2xx; every
`false` → write returns 403 with `*acl.ForbiddenError`. Parameterized over all
three ACL implementations. Verdict bool, not reason equality. Integration test.
4. **AC4 — list endpoint.** List endpoint returns per-row `_actions` on each
item plus top-level `_actions.create` for the collection. Handler test.
5. **AC5 — frontend consumption.** Buttons consult `entity._actions[verb]`.
False → omit. True → render. Absent → render + dev-mode warning if session is
authenticated. Component unit tests + E2E.
6. **AC6 — additive vocabulary.** Emit a synthetic verb `noop`; existing
component tests pass unchanged with no console error.
7. **AC7 — AWM6L payoff.** Read-only mode produces a button-less data-entry
UI driven entirely from the backend `_actions` map. E2E.
8. **AC8 — no audit noise on read path.** GET request produces zero audit
records under all three ACLs. Unit test against `audit.NewMemory()`.
9. **AC9 — scope-of-invariant (documentation-only).** Writes via MCP / Lua /
scheduler can succeed for verbs whose `_actions` would be `false` for the SPA
principal. Expected; flagged as documentation-only.
10. **AC10 — structural same-code-path.** Grep test asserts
`acl.WriteRequest{Op:` appears only in `internal/dataentry/affordances.go`
(`translateVerb`) and test files.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

Ten-system research sweep at `.ignored/action-affordances-research.md` (4528
words). Highlights:

- **Convergent design across four independent products** (Cerbos PDP, GitHub
REST `permissions`, Plone `@actions`, Kubernetes SSAR). All landed on inline
`{verb: bool}` map per resource.
- **Anti-pattern shortlist** (research §Anti-patterns): predicates on the
wire, role-and-action conflation, affordance endpoint as security check,
SSRR-style enumeration.
- **Plone (closest prior art)** — 20+ years' lesson: ship the resolved
verdict only.
- **Codebase reuse.** `V1Entity._actions` already exists as
`*V1Actions{Delete, Transitions[]}`. Phase 1 reshapes it in place to
`map[string]bool`. The legacy `transitions[]` array is empty until ACL v0.5
lands transition verbs — no functional loss.
- **ACL primitive.** `acl.WriteRequest{Op, EntityType, RelationType}` from
ACL v0 backs the computation. `Op` enum is `OpCreate/Update/Delete/Rename`.
Phase 1 verb vocabulary matches this exactly.

Design synthesised in `.ignored/action-affordances-design.md` (v2, audit + crit
findings folded in).

**Reviews performed:**

- **go-architect** (round 1) — 3 critical, 5 significant findings on package
placement, structural enforcement of same-code-path, arch-lint compliance.
- **cranky-code-reviewer** (round 1) — 5 critical, 7 significant findings on
verb-vs-Op mismatch, cardinal-rule scope, anonymous overload, AC3
unimplementability, fictitious `policy_revision`, dropped batching, missing
risks.
- **User crit** (rounds 1-2) — chose closed-world `{verb: bool}` over
`[allowed_verbs]`; dropped parallel-emit (`_actions_v2`); dropped
`/meta/actions` discovery endpoint. Round 3: approved.

All critical+significant findings folded into design v2 and this plan. The
design simplifications from crit (no parallel-emit, no discovery endpoint)
collapsed phasing from 5 phases to 2.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Wire shape.** Reshape `_actions` on `V1Entity` from
`*V1Actions{Delete, Transitions[]}` to `map[string]bool`. Add `_actions` on
`V1ListResponse` for collection-scope verbs (just `create`). Backend reshape and
SPA migration ship in the **same PR**.
2. **Computation lives in `internal/dataentry`.** New file
`internal/dataentry/affordances.go`:
   - `translateVerb(verb, entityType) (acl.WriteRequest, bool)` — the single
source of truth used by both the serializer and the write handlers.
   - `verbsForType(s *AppState, entityType) []string` — metamodel-derived
verb list. Stays in `dataentry` so `acl` doesn't import `metamodel`.
   - `computeActions(ctx, s, e)` — per-row map computation.
   - Reserved `computeActionsBatch(ctx, s, entities)` signature for future
optimization.
3. **Same-code-path made structural.** Both `computeActions` and write
handlers route `acl.WriteRequest` construction through `translateVerb`. Grep
test (AC10) asserts no other site constructs `WriteRequest{Op:`.
4. **Phase 1 verb set is closed:** `{create, update, delete, rename}`.
Matches ACL v0's `Op` enum exactly.
5. **Snapshot once per request.** Handler captures `s := a.State()` at the
top; `computeActions` reads only from `s` for the verb list.
6. **No cache in phase 1.** Profile gate in phase 2: benchmark list response
at 100/1k/10k entities × 3 verbs. If p95 > 200ms with `Declarative`, cache lands
with key `(principal_id, entity_id, entity_updated_at)`, TTL 60s. No
`policy_revision` field.
7. **Anonymous fallback.** Anonymous principals get `_actions` omitted
entirely. Authenticated principals always get the field present (possibly `{}`
if all denied). SPA emits dev-mode warning when authenticated session receives
missing field — distinguishes anonymous-fallback from serializer-bug.
8. **Verb naming rule.** Phase 1: single bare nouns matching `Op` constants
— no colons, no arguments. Multi-token verbs deferred to ACL v0.5.

**Alternatives considered (and rejected):**

- **HAL `_links`, JSON:API `meta.permissions`, OData metadata DSL** — research
§1–3.
- **Frontend `useACL()` mirror** — TKT-AWM6L's plan; killed by crit
("duplicating the ACL logic on the frontend").
- **GraphQL `viewerCan*`** — nullability ambiguity (research §10).
- **Stripe attempt-and-recover** — only for coarse roles.
- **Eager bake-in cache** — profile first.
- **Parallel-emit `_actions_v2`** — crit round 2: legacy not needed; reshape
in place.
- **List-of-allowed-verbs `[v1, v2]` instead of `{v: bool}`** — crit round 1:
closed-world map is testable; list collapses *denied* / *not evaluated* / *not
defined* / *old server* into the same wire shape.
- **`/api/v1/meta/actions` discovery endpoint** — crit round 2: collection
`_actions.create` covers create-menu population on list responses; no separate
endpoint needed.
- **`ComputeActionMap` in `internal/acl`** — architect C2.
- **`ListVerbs(entityType)` in `internal/acl`** — architect C1.
- **`OpTransition` / `Subject` / `Principal` on `WriteRequest` for phase 1**
— cranky #1. Defer transition/relation verbs to TKT-Y72A-followup gated on ACL
v0.5.

**Files to modify:**

- `internal/dataentry/affordances.go` (NEW) — `translateVerb`,
`verbsForType`, `computeActions`, reserved batch signature.
- `internal/dataentry/api_v1.go` — reshape `_actions` on `V1Entity` from
`*V1Actions{Delete, Transitions[]}` to `map[string]bool`. Add `_actions` on
`V1ListResponse`.
- `internal/dataentry/handlers_api.go` — wire `computeActions` into
per-entity, per-list, per-collection serialize. Route write handlers'
`WriteRequest` construction through `translateVerb`.
- `frontend/src/types/entity.ts` — change `_actions` type from legacy
`EntityActions{delete, transitions}` to `Record<string, boolean>`.
- `frontend/src/components/**` — migrate existing `_actions.delete.allowed`
and `_actions.transitions` access patterns to `_actions.delete` and (deferred)
`_actions['transition:*']`. Add dev-mode warning for authenticated-missing.
- `docs/api.md` — document `_actions` map, verb vocabulary.
- `e2e/` — read-only-mode-has-no-write-buttons scenario.

**No changes to `internal/acl/`.**

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Principal from request.** Same source as today
(`defaultPrincipalResolver`).
- **Verb name.** Closed set in `translateVerb`: `create/update/delete/rename`.
No interpolation from request data.
- **No client-supplied verb in `_actions` computation.** Server-determined.

**Security-Sensitive Operations:**

- **The cardinal rule.** The action map is *not* authorization. The write
endpoint must re-authorize. AC3 asserts the bidirectional contract; AC10
enforces it structurally via the grep test.
- **Scope of the invariant.** HTTP write endpoints reached by the SPA only.
MCP / Lua / scheduler write paths bypass `_actions` (re-authorized at the
`entitymanager.Manager` boundary). AC9 documents.
- **Anonymous recon prevention.** `_actions` omitted for anonymous principals.
No separate `/meta/actions` endpoint to enumerate from.
- **No predicates / TALES / role names on the wire.** Verdicts only.
- **Same code path as enforcement — structural.** `translateVerb` is the
shared constructor (architect C3 fix).
- **No audit-log noise from read path.** `computeActions` calls
`AuthorizeWrite` for verdict; no audit records. AC8 asserts.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test scope | Location |
|---|---|---|
| AC1 | Read-only handler test | `internal/dataentry/handlers_api_test.go` |
| AC2 | NopACL handler test | `internal/dataentry/handlers_api_test.go` |
| AC3 | Bidirectional contract across all three ACLs | `internal/dataentry/affordances_contract_test.go` (new) |
| AC4 | List endpoint per-row + top-level `_actions` | `internal/dataentry/handlers_api_test.go` |
| AC5 | Component unit tests + dev-mode warning | `frontend/src/**/__tests__/` |
| AC6 | Synthetic verb `noop`; existing tests pass | `frontend/src/**/__tests__/` |
| AC7 | E2E: read-only data-entry has no write buttons | `e2e/specs/read-only-mode.spec.ts` (extend) |
| AC8 | GET produces zero audit records under all ACLs | `internal/dataentry/affordances_test.go` |
| AC9 | Documentation only | `docs/api.md` |
| AC10 | Grep test for `acl.WriteRequest{Op:` outside `affordances.go` | `internal/dataentry/lint_test.go` (new) |

**Edge Cases:**

- **Anonymous principal.** `_actions` omitted. SPA falls through to "show all
  + no warning."
- **Authenticated, all denied.** `_actions: {}`. SPA renders no buttons.
- **Authenticated, missing field (bug).** SPA renders all + dev-mode warning.
- **Verb computed `true` at fetch, state changes before write.** Server
re-authorizes; 403; SPA error toast.
- **List with mixed-permission rows.** Each row carries its own `_actions`.
AC4.
- **Type not in metamodel** (orphan markdown): `verbsForType` returns nil;
`_actions` field omitted (or empty, depending on serializer choice).
- **Policy reload mid-request.** Snapshot at handler entry; subsequent
requests see new state.
- **Large verb maps.** Phase 1's 3-verb-per-item set keeps payload cost
trivial.

**Negative Tests:**

- **Unknown verb in `translateVerb`.** Returns `(WriteRequest{}, false)`.
Caller skips.
- **Nil principal.** Anonymous fallback — `_actions` omitted.
- **GET as `Declarative` ACL with empty policy.** `_actions`: all denied.
SPA hides all buttons.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| **Wire-vs-policy drift** | Shared `translateVerb`; bidirectional contract test (AC3); grep test (AC10). |
| **Snapshot skew (true→403)** | Acknowledged; SPA error toast + dev-mode log for measurement. |
| **N+1 on list endpoints** | Snapshot once. Phase 2 profile gate at 100/1k/10k; cache lands if p95 > 200ms. Key `(principal_id, entity_id, entity_updated_at)`; no `policy_revision`. |
| **Verb vocabulary outstrips ACL v0** | Phase 1 verbs = `{create, update, delete, rename}` exactly. `transition:*`/`relation:*` deferred to TKT-Y72A-followup. |
| **Existing `_actions` callers break on reshape** | rela's SPA is the only consumer; reshape + SPA migration in same PR. Pre-implementation step: grep `frontend/`/`e2e/` for current `_actions.delete`/`_actions.transitions` access; migrate each site. Legacy `transitions[]` was empty without ACL v0.5 — no functional loss. |
| **Audit-log noise from read path** | `computeActions` calls aren't writes; AC8 asserts zero records. |
| **SPA caches stale `_actions`** | Existing 1-min entity-cache TTL + SSE invalidation. Policy-changed SSE deferred (design Q5). |
| **ACL v1 snapshot threading** | Flagged in design Q4. Phase 1 doesn't commit to a v1 signature. |
| **Verb ownership / churn** | Phase 1 list closed. Additions need `translateVerb` + ACL Op + doc PR. Renames need major API version bump. |

**Effort:** xl (matches ticket effort). Phase 1 + 2 over two PRs; optional
phase-3 follow-ups as separate PRs.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] **API docs** — `docs/api.md` gets an "Action affordances" section:
`_actions` shape, four-verb vocabulary, anonymous fallback, verb-naming
guidance.
- [x] **CLAUDE.md** — new rule: "Action affordances are UI hints; the write
endpoint must re-authorize. New write paths in `internal/dataentry` route
`WriteRequest` construction through `translateVerb`."
- [x] **`docs/security.md`** — note that read-only-mode UI hiding is
data-driven (via `_actions`). Document scope of invariant.
- [x] ~~User guide / reference docs~~ (N/A: backend wire-shape change with no end-user feature surface; the SPA itself is what users see and that's covered by api-reference.md)
- [x] ~~CLI help text~~ (N/A: no CLI changes in this ticket)
- [x] ~~README.md~~ (N/A: project-level positioning unchanged)

## Design Review

- [x] Run `/design-review` before starting implementation — **architect +
cranky audits round 1; user crit rounds 1-2 with directional simplifications;
round 3 approved**
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Three review cycles before transition to
in-progress.

- **go-architect (round 1)**: C1/C2/C3 (package placement, structural
same-code-path), S1-S5 (reshape rollback, verb syntax, `/meta/actions`
semantics, ACL v1 snapshot, phase 5 coverage). All folded.
- **cranky-code-reviewer (round 1)**: #1-12 (verb-vs-Op, invariant scope,
workflow-vs-ACL conflation, fictitious `policy_revision`, anonymous overload,
verb ownership, recon, reshape rollback, dropped batching, AC3
unimplementability, missing risks, dropped research recommendation) + 13 minor
items. Criticals + significants folded.
- **User crit (rounds 1-3)**: round 1 — chose `{verb: bool}` map shape over
list-of-allowed (closed-world, testable, future-evolution headroom). Round 2 —
dropped parallel-emit (`_actions_v2` removed; reshape in place)
  + dropped `/meta/actions` (collection `_actions.create` covers menu
population). Round 3 — approved.

The simplifications from crit shrank the rollout from 5 phases to 2 + optional
follow-ups.
