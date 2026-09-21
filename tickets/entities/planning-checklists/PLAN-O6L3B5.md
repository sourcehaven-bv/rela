---
id: PLAN-O6L3B5
type: planning-checklist
title: 'Planning: Duplicate an entity from the detail page: relation-picker modal, then a prefilled create form'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** Full in/out scope lives in TKT-Z8K2FS under "Scope". Summary: a
Duplicate action on the entity detail page opens a modal of relation-type
checkboxes (both directions), then an embedded create form prefilled from the
source; submit creates the copy and opens it. Out: per-edge selection, deep
duplication, edge properties, attachments/comments, bulk duplicate, new
`_actions` verbs.

**Acceptance Criteria:** 26 criteria in TKT-Z8K2FS, each naming its assertion
site. Started at 15, grew to 20 during planning, then to 26 through design
review. One (the incoming-edge partial failure) was DELETED as a solution to a
non-existent problem — see RR-LOWNMB.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — no RES entity. Three targeted codebase surveys were run
instead; their findings are recorded in the ticket rather than a separate doc,
because every conclusion is a fact about this repo, not an external survey.

**Existing Solutions:**

- **Libraries:** none applicable. This is entirely repo-internal UI + wiring.
- **`handleV1CloneEntity`** (`write_handler.go:1183`) — a shipped, orphaned,
  relation-blind clone. Superseded, not extended; see the ticket.
- **The `copies:` kernel** (`internal/entitymanager/copy.go`) — face-to-face
  declared copies. Wrong mechanism (write-first, invoke-by-name), right
  architecture. Considered and rejected in the ticket.
- **`inline_create`** (`responses.go:803-821`) — the enablement signal, already
  encoding "may create AND a form resolves" and shipping the form id. Replaced the
  planned `_duplicate` affordance key entirely (RR-ULJ0WK).
- **`SectionCreate`** (`config.go:1467-1507`, TKT-R4BMJM) — the config-block
  precedent: absence means default, allowlists refused at load, no second
  spelling of one meaning.
- **`applyTemplate`** (`DynamicForm.vue:935-955`) — already writes typed
  properties + content + relations and re-baselines `originalData`. A duplicate
  candidate is a template computed from an entity.
- **`resolveDirection`** (`relations_direction.go:41-57`) — the batched create
  already writes both edge directions atomically under inverse-named keys, which
  removed a planned post-create write path (RR-LOWNMB).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Fully documented in TKT-Z8K2FS. The three decisions that
were open at ticket creation and are now settled:

1. **Prefill transport = props to a modal-embedded `DynamicForm`.** The URL
   channel was rejected on evidence: it drops list properties entirely
   (`DynamicForm.vue:680`), coerces nothing, has no content channel, and exceeds
   realistic proxy limits on mean dogfood data (~3 KB markdown → 6-9 KB encoded).
   `EntityDetail.vue:809-828` already states the rule: page navigates with
   query params, modal passes props.
2. **Config home = `entity_views.<type>.duplicate`**, not `views:`. The ticket's
   original YAML was invalid — `views:` is keyed by view id, not entity type.
3. **Clone endpoint = superseded, removed in a follow-up ticket**, not extended
   and not deleted here.

**Files to modify:**

- `internal/dataentryconfig/config.go` — `DuplicateConfig` + field on `EntityViewConfig`
- `internal/dataentryconfig/validate.go` — validator modelled on `:2311-2317`
- `internal/dataentryconfig/validate.go` — relax the empty-`detail_view` check
- SPA plumbing for `entity_views` (type, store field, reader) — none exists today
- `frontend/src/api/entities.ts` — client fn for the all-relations endpoint
- `frontend/src/components/entity/EntityDetail.vue` — button, desktop + overflow
- `frontend/src/components/entity/DuplicateModal.vue` — new
- `frontend/src/components/forms/DynamicForm.vue` — `duplicateFrom` prop channel
- `docs-project/entities/guides/GUIDE-data-entry.md` — source, not the generated file
- tests: `internal/dataentryconfig/`, `internal/dataentry/querybudget_test.go`, `e2e/tests/`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Selected relation types** (user, from the modal). Allowlist: the create path's
  `gateCreateRelationAffordances` (`write_handler.go:300`) re-authorizes every
  edge before `CreateEntity`, so a forged type is refused regardless of what the
  modal offered. Asserted on both directions (AC12) — one gate, two inputs, since
  incoming edges ride the same create body under inverse-named keys.
- **Source entity id** (user, from the route). Already read-gated; a hidden source
  is indistinguishable from a 404 (AC13).
- **`duplicate.properties`** (operator config). Allowlist validated at load; an
  unknown property name is a load error, matching `SectionCreate` doctrine.
- **Prefilled property values** (server, already redacted by the entity GET).
  Excluded from the prefill: `file`-typed properties (dangling attachment path,
  RR-4650VR) and machine-typed properties (a create is an entry, RR-IKIYM1).

**Security-Sensitive Operations:**

- **Read gating.** The modal's relation list comes from an endpoint that already
  drops edges whose peer the caller cannot read (`relation_visibility.go:69`), so
  a user can only copy what they may see. Peer visibility can change between
  modal-open and submit; the create path's own gate is the backstop and the
  failure must surface, not silently drop an edge.
- **Redaction disclosure.** A duplicate of an entity with `_redacted` properties
  is incomplete by construction. The user is told which properties could not be
  carried. This is disclosure of property *names*, which the project explicitly
  does not treat as secret (the metamodel is served over the API), not values.
- **No new write path.** The duplicate submits through the existing batched
  create, inheriting its authorization, validation and atomicity.
- **Not a redacted read-modify-write.** A duplicate creates a new entity rather
  than overwriting an existing one, which is why "all visible properties by
  default" is legal here while `copies.go:120-132` refuses `fields: all`
  cross-entity. Reasoning recorded in the ticket.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Each of the 26 ACs names its assertion site. Layers:

- **Config unit tests** (`internal/dataentryconfig`) — AC9, AC10: valid block
  narrows; unknown property, non-mapping, empty list each fail load.
- **Handler/affordance tests** (`internal/dataentry`) — AC1 (verdict-driven
  presence/absence), AC12, AC13 (forged requests), AC14 (budget at 10 and 50 rows
  via `newBudgetApp`, `querybudget_test.go:162`).
- **Frontend unit tests** — AC2/AC3 (list construction, default check state),
  AC4 (empty state), AC10a (redaction disclosure), AC10c + AC20 (depth cap,
  modal-stack), AC19/19a (prefill survives the commit filter and the two relation
  pruners).
- **E2E** (`e2e/tests/`) — AC21 full flow, AC5-AC8, AC11, AC15, AC16, AC19b.

**Edge Cases:**

- Entity with zero visible relations → explicit empty state, never an empty list
  with a live Confirm (AC4).
- Entity where every peer of a type is hidden → the type does not appear (AC2).
- Self-referencing entity with a relation and its inverse both checked →
  `detectSelfLoopShapeConflict` (`relations_direction.go:89`) (AC18).
- Faced entity → lands on the source's face; for a faced type an omitted face is
  a refusal, not a default (AC11).
- Entity with redacted properties → named, not silently dropped (AC10a).
- Non-string property types: list, number, boolean, date (AC15).
- Unique-property collision, edited and unedited (AC8).
- Incoming-edge write failing after the entity exists (AC17).

**Negative Tests:** forged relation type refused on both directions against a
schema with an explicit non-creatable verdict — a verdict-free schema passes
vacuously since undeclared types are default-permissive (AC12); forged source id
→ 404-shaped (AC13); each invalid config shape → load error (AC10); unedited
colliding unique title → 422 surfaced as a field error, never a silent failure
(AC8).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Five in the ticket with mitigations: affordance/write divergence,
incoming-edge partial failure, inverse relation keys, relation enumeration cost,
peer visibility recheck. Plus two implementation constraints that fail CI if
missed: `App` plimsoll headroom is zero, and `docs/data-entry.md` is generated.

**Effort:** l (unchanged). Backend is small — no new write path, one config block,
one affordance key. The weight is frontend: a new modal, the `DynamicForm` prop
channel, and the desktop/overflow duplication.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — **via `docs-project/entities/guides/GUIDE-data-entry.md`**,
      never edited directly. Document the Duplicate action and the
      `entity_views.<type>.duplicate` block. Run `just docs` twice, confirm no diff.
- [x] ~~docs/metamodel.md~~ (N/A: config lives in data-entry.yaml, not schema.yaml)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI surface)
- [x] ~~CLAUDE.md~~ (N/A: no new convention; reuses existing patterns)
- [x] ~~README.md~~ (N/A: no project-level change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 10 review-responses, all `addressed`.

Critical: RR-9QGFP2 (inline-create depth cap fails open — the modal must call
`provideInlineCreateDepth()` or it ships the modal-in-modal the codebase forbids),
RR-ULJ0WK (`_actions.create` does not exist on an entity response; `create` is a
collection-scope verb — use the sidebar `inline_create` map), RR-DDY9LG (the
commit filter silently drops untouched prefilled properties, which is the
"short copy produced silently" outcome AC10a forbids, via a channel AC10a did not
cover).

Significant: RR-LOWNMB (the incoming-edge partial-failure design solved a
non-existent problem — the batched create writes both directions atomically;
building it would have INTRODUCED the non-atomicity feared), RR-IKIYM1
(state-machine prefill is silently replaced, not rejected), RR-4650VR (`file`
properties would carry a dangling attachment reference), RR-LDPV50 (a
`duplicate:`-only `entity_views` entry fails load today), RR-VR2YGE
(RR-7Z3SFC's single-peer fix does not generalize to an N-peer prefill).

Minor: RR-EP785R (AC4 was a disjunction, so it deferred a design decision into an
acceptance criterion), RR-13WJHZ (AC12 reasoning, `unique:`+`list:`, a wrong
citation, and AC14's budget target).

Net effect: acceptance criteria went from 20 to 26; two planned pieces of work
were removed as unnecessary (the `_duplicate` affordance key, the post-create
incoming-edge write path); three silent-data-loss paths were closed before any
code existed.
