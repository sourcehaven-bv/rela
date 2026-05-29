---
id: PLAN-VF3U
type: planning-checklist
title: 'Planning: Create-form field affordances: default _fields verdicts for an unsaved entity'
status: done
---

<!-- @managed: claude-workflow v1 -->

Full plan: `.ignored/TKT-3I5U-plan.md` (working doc). Summary below. Design
review complete: 8 findings (RR-R8OR, RR-4O6E, RR-SIA6, RR-Y85M significant;
RR-7PL4, RR-ZKL2, RR-HUQ3 minor; RR-YP8R nit) — all addressed. External-systems
research validated the pattern (value-dependent verdicts re-derived server-side;
UI-hint-only, re-authorize on commit).

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (below)
- [x] Acceptance criteria documented with test scenarios

**Problem:** After BUG-Q60V the v1 create path 403s denied fields, but the SPA
create form has no affordance source (renders everything in create mode;
`_fields` only rides on a fetched entity). Users fill fields the server then
rejects.

**Core insight:** Affordance verdicts depend on the candidate entity's *field
values* (predicates read them), so a static type-default fetch is wrong for
value-dependent `when:` and fails open. Sparse `_fields` conveys "hidden" via
absence-from-`_fields`+absence-from-`properties` — impossible without an
instance. Both dissolve if create has an instance → model create as editing a
staged entity.

**Scope:**
- IN: model create as editing a staged `++new++` entity (form-only sentinel); unify create/edit field-filter path; live affordance re-derivation via a `?dry_run=true` create call (verdict-only, no persist, no audit, no writeMu); fields + options + warnings.
- OUT: staged-relation validation (relations gated at commit only); edit-mode reusing the dry-run endpoint (its own PR, no abstraction ahead of need); server-persisted drafts.

**Acceptance Criteria:**
1. Create form renders a policy-denied field disabled (read-only), not editable-then-403.
2. Create form omits a hidden field entirely, no flicker.
3. Create form filters disallowed enum options out of the select.
4. Value-dependent verdicts update live (filling A re-derives B within debounce).
5. Committing a clean staged entity creates it (201); commit body sends only visible-writable keys; hidden/read-only defaults applied server-side.
6. Sentinel `++new++` never appears in any outbound request body (unit test).
7. No regression to edit mode (shared filter path).
8. Commit re-authorizes regardless of dry-run: a denied field POSTed directly still 403s (BUG-Q60V tests pin this).

## Research

- [x] Checked codebase for reusable patterns + external systems

**Codebase (file:line):** PATCH response carries `_fields`+`warnings` via
`serializeEntityForWire` (`api_v1.go:747-749`); autosave debounce +
AbortController stale-drop (`useAutoSave.ts`, edit-only today);
`/_templates/<type>` precedent (`api_v1.go:2517`); BUG-Q60V empty-ID candidate
gate (`api_v1.go:~482`); defaults applied in `entitymanager.CreateEntity` AFTER
the gate (`core.go:66-80`); edit-mode filter +
`isFieldReadonly`/`optionVerdictsFor` (`DynamicForm.vue:~145-175`).

**External systems:** Salesforce Dynamic Forms (value-dependent, client, falls
back to save-time when server data needed), JSONForms `rule`/Sanity
callbacks/React Admin FormDataConsumer (value-dependent, client, NOT auth),
Django/DRF (static-per-user, obj=None on create), Strapi/SAP RAP
(server-persisted drafts). Synthesis: our value-dependent + server-authoritative
fusion is novel-but-sound; the round-trip is the principled fix for the
degradation others hit; client-staging matches form-centric-admin norm.
Universal rule: UI hint only, re-authorize on commit.

## Approach

- [x] Approach chosen, builds on existing patterns, alternatives documented

**Server:** `?dry_run=true` early-return in `handleV1CreateEntity`, BEFORE
`a.writeMu.Lock()`. Snapshots `a.State()` once (read-shaped). Verdict-only path
computes `_fields`/`_relations`/`warnings` (shared "compute verdicts" step,
split from "enforce+audit"); never calls `denyAffordance`/`auditSink`; skips
`CreateEntity`. Response `Cache-Control: no-store`, no ETag. Commit (real
create) is the sole authorization point and the sole audit row.

**SPA:** staged `Entity{type, properties: visible-field defaults}`, internal id
`++new++` (form-only; `isStaged()`+`STAGED_ID`; stripped before every request).
Remove `isEdit` render-everything fork → both modes filter against current
entity's `_fields`+`properties`. Populate staged `_fields` on mount (blank eval,
first paint gated, F19 anti-flicker) + debounced on change (reuse autosave
AbortController stale-drop). Map warnings/`rule_id` to per-field feedback;
block-submit on hard denials (UX only). Fail-open if dry-run errors. Commit =
existing `POST create`, sending only visible-writable keys.

**Alternatives rejected:** static type-default fetch (wrong/fails-open); new
`hidden[]` wire shape (unneeded with an instance); end-to-end sentinel
(persist-guards, fake-but-present `entity.id`; only for server drafts).

**Files:** `internal/dataentry/api_v1.go` (dry-run early-return + tests);
`frontend/src/components/forms/DynamicForm.vue` (staged model, remove fork,
mount+change eval, send-only-visible); `frontend/src/composables/useAutoSave.ts`
or sibling (staged debounce/stale-drop); SPA api client;
`docs/data-entry/api-reference.md`.

## Security Considerations

- [x] Input sources + validation identified

Staged properties = same trust as a normal create body. Dry-run persists nothing
(test asserts store unchanged) and emits no audit. Commit re-authorizes
(BUG-Q60V gate). Sentinel never on the wire. Relation/entity-scoped predicates
fail closed for the candidate (no ID/edges). Hidden/read-only defaults applied
server-side, never through the gate as user writes. Dry-run verdicts advisory
only — client cannot bypass auth by ignoring them.

## Test Plan

- [x] Scenarios per AC; edge cases; negative cases

**Server:** dry-run returns correct shapes, persists nothing, emits no audit,
takes no write lock; table-driven
hidden/read-only/enum-filtered/value-dependent/allowed; commit-without-dry-run
still gates (ref BUG-Q60V tests). **SPA (vitest):** disable read-only, omit
hidden (no flicker), filter options, live re-derivation on dependent change,
`++new++` stripped from bodies, commit posts only visible-writable keys + 201,
dry-run 500 → form usable, type-A-then-B → only B applies, edit parity
unaffected. **Edge:** blank required fields bind Nil (no Eval error);
query-param prefills feed initial eval; hidden field with default → omitted from
body → default lands server-side → 201.

## Risk Assessment

- [x] Risks + mitigations; effort = m

Round-trip per edit (debounced+stale-drop, submit-once form); dry/real drift
(shared validation path, one early-return); sentinel leak
(form-only+strip+test); staged relations deferred (commit-gated); audit flood
(verdict-only path, no audit on dry-run); writer-lock contention (read-shaped
dry-run).

## Documentation Planning

- [x] User-facing docs identified

`docs/data-entry/api-reference.md`: document `?dry_run=true` create mode +
staged-create behaviour. Docs-checklist on entering implementation
(enhancement).

## Design Review

- [x] Run `/design-review` before implementation
- [x] All critical/significant findings addressed (RR-R8OR, RR-4O6E, RR-SIA6, RR-Y85M all addressed; minors/nit addressed)

**Design Review Findings:** RR-R8OR, RR-4O6E, RR-SIA6, RR-Y85M, RR-7PL4,
RR-ZKL2, RR-HUQ3, RR-YP8R
