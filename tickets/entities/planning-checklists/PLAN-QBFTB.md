---
id: PLAN-QBFTB
type: planning-checklist
title: 'Planning: Multi-step (wizard) forms with conditional steps'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope (this ticket, TKT-CHLAJ):
- An optional `steps:` layout on an admin-authored form in `data-entry.yaml`. Each step has a `title` and its own `fields:`/`relations:` (the existing field/relation config, unchanged).
- Per-step and per-field `visible_when:` / `required_when:` conditions, each a **single expression string** in the shared condition grammar (TKT-BL7XZ), referencing earlier field values (`form.<field>`). Evaluated client-side by the shared engine.
- Next/Back navigation in the SPA; per-step validation gates Next; full-form validation on final Submit.
- The active step index encoded in the URL query (`?step=N`), refresh/deep-link safe, using the existing `useUrlFilterSync` pattern (`router.replace` + echo suppression).
- Wizard is opt-in per form: a form with `steps:` renders as a wizard; a form with `fields:`/`relations:` (no `steps:`) renders single-page exactly as today.

DEPENDENCY: the condition grammar + evaluator is a **separate ticket,
TKT-BL7XZ** ("Client-side condition expression engine", implements FEAT-8Z47U).
TKT-CHLAJ `depends-on` TKT-BL7XZ and is its first consumer. TKT-BL7XZ must land
first. This split was chosen because a shared expression engine also serves ACL
button enable/disable and view-section visibility — see the "Approach" note and
FEAT-8Z47U.

OUT of scope:
- The expression engine itself (own ticket TKT-BL7XZ).
- Server-side (Go) evaluation of `visible_when`/`required_when` — conditions run only in the browser (see Security). The server keeps its existing write-time validation (soft warnings + 422 for structural-impossible) unchanged.
- Wizard-specific autosave semantics in edit mode beyond what already exists.
- The frontend's latent `sections?` concept — superseded by `steps:` (retired in this PR to avoid two layout concepts).

**Acceptance Criteria:**

1. A data-entry form can be configured with ordered, titled steps.
   - Test: author a form with `steps: [{title: A, fields: [...]}, {title: B, ...}]`; SPA shows a stepper with A then B; only step A's fields visible initially.
2. A step or field can be shown/hidden and made required/optional based on the value of an earlier field in the same form.
   - Test: step B `visible_when: "form.has_processors == true"` — hidden until the toggle is on; a field `required_when: "form.has_processors == true"` — Next/Submit blocks only when the toggle is on and the field is empty. OR case: `visible_when: "form.has_processors == true or form.is_joint_controller == true"`.
3. Next/back navigation works; per-step validation blocks "next" on invalid input; full validation runs on submit.
   - Test: leave a required field on step A empty → Next blocked with per-field error; fill it → Next advances; Back returns without losing entered values; Submit re-validates all *visible* steps.
4. The current step is encoded in the URL so refresh/deep-link returns to it.
   - Test: advance to step 3, refresh → still on step 3. Only the step index is URL-encoded (entered create-mode values are not persisted across refresh — document this). Invalid `?step=99` → first step.
5. Single-page forms keep working unchanged; wizard mode is opt-in.
   - Test: existing forms (no `steps:`) render and submit identically; e2e regression suite green.

## Research

- [x] For larger features: run `/research` — N/A (scope well understood; grammars surveyed inline; engine split into FEAT-8Z47U)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — medium enhancement; prior art surveyed via three Explore
agents (Go config, Vue form, condition-grammar + URL-sync).

**Existing Solutions:**

- **Reference impl (OpenVWR / Filament Wizard):** thin wrapper over Filament `Wizard` with `persistStepInQueryString`; per-register step definitions; conditional fields via toggles + `FormHelper::isFieldEnabled`; a `register_layout` preference switches steps-vs-one-page. Confirms the shape: ordered steps, step-in-URL, conditional fields, opt-in layout, and hidden-branch fields excluded from persisted data.
- **Form config (Go):** `internal/dataentryconfig/config.go` — `Form` (line 132), `FormField` (151), `FormRelation` (164). Served verbatim at `GET /api/v1/_config` (`handleV1Config`, `internal/dataentry/api_v1.go:1872`); the config structs' own `json:` tags ARE the wire contract (`internal/apiwire/v1/responses.go:189` types `Forms` as `map[string]dataentryconfig.Form`). Author-defined YAML, not metamodel-derived. Validated by `validateForms` (`internal/dataentryconfig/validate.go:220`).
- **Latent frontend seam:** `frontend/src/types/config.ts:55` already declares `FormConfig.sections?` and `DynamicForm.vue` (~146-154, 1091-1093) already flattens/renders them — but the Go backend never emits `sections`. Dead scaffolding. We introduce `steps:` (one-at-a-time) rather than reuse `sections` (stacked); the flatten logic generalizes to `steps.flatMap(...)`.
- **Form rendering (Vue):** `views/FormView.vue` → `components/forms/DynamicForm.vue` (~1490-line engine). Config from Pinia `schemaStore` (`stores/schema.ts`), loaded once on mount. Form state in **local refs** in DynamicForm: `formData`, `relations`, `content`, `errors` (keyed by property). Client `validate()` = required/type/enum over currently-shown fields. Server 422 via `ApiError.validationErrors` mapped into `errors`. Create commits on explicit Save (`entitiesStore.create`); edit autosaves per-field (`useAutoSave`). Widget selection: `widgets/registry.ts` `defaultWidgetFor`.
- **Condition grammar decision (see FEAT-8Z47U + TKT-BL7XZ):** rather than the flat `internal/filter` string grammar (AND-only) or porting the Go `internal/predicate` Lua-subset engine (~1,400 LOC, depends on gopher-lua's parser, would be a second implementation to keep in sync), we build a small **hand-written client-side expression engine**: a Pratt/recursive-descent parser (~150-250 lines) + tree-walk evaluator (~300-450 lines TS total). It gives full `and`/`or`/`not`, comparisons, parens, dotted refs, and a host-function hook — reusable across forms, ACL button enable/disable, and view visibility. Grammar kept congruent with `internal/predicate` (`==`/`~=`/`and`/`or`) so a future server-eval path is a drop-in. Rationale for the split: the user flagged the same condition requirement recurring across features, so the engine is scoped as its own reusable ticket rather than a throwaway helper inside forms.
- **URL-sync:** `frontend/src/composables/useUrlFilterSync.ts` — seed synchronously from `route.query` on setup (refresh-safe), write via `router.replace` (no history spam), signature-based echo suppression. Template for a `step` param.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Decision (from user): use a **shared hand-written client-side expression
engine** (TKT-BL7XZ / FEAT-8Z47U) for conditions. `visible_when`/`required_when`
are single expression strings. This ticket consumes that engine.

Config shape (authored in `data-entry.yaml`):
```yaml
forms:
  processing_record:
    entity_type: processing-record
    title: New processing record
    steps:
      - title: Controller
        fields: [ ... ]                       # same FormField entries as today
      - title: Processor
        visible_when: "form.has_processors == true"
        fields:
          - property: processor_name
            required_when: "form.has_processors == true"
        relations: [ ... ]
      - title: DPIA
        # WP248 chain / OR both natural in the engine:
        visible_when: "form.q1 == 'no' or form.q2 == 'no'"
```

Go side (`internal/dataentryconfig`):
1. Add `Steps []FormStep` to `Form` (JSON `steps,omitempty`). `FormStep{Title string; VisibleWhen string; Fields []FormField; Relations []FormRelation}`. Add `VisibleWhen string` (json `visible_when,omitempty`) and `RequiredWhen string` (json `required_when,omitempty`) to `FormField` (and `FormRelation`). Conditions are opaque strings to Go (the engine lives in the SPA); Go does not evaluate them.
2. `validateForms` (validate.go): a form has EITHER `fields`/`relations` (flat) OR `steps`, not both (error if both non-empty). Each step's fields/relations validated with the existing per-field/relation checks (reuse the loop). Condition strings: Go does a **light** check only — non-empty when present; deep grammar validation is the engine's job client-side. (Optional stretch: a tiny Go-side syntactic sanity check, but avoid duplicating the parser — deferred.)
3. `handleV1Config` relation-widget auto-resolution loop must also walk `Steps[].Relations`.

Frontend side:
4. `types/config.ts`: add `steps?: FormStep[]` + `visible_when?: string`/`required_when?: string` on step/field/relation types. Retire the unused `sections?` type.
5. `DynamicForm.vue` consumes the engine (`evaluate(expr, {form: formData, entity, current_user})`):
- `allFields`/`fields` generalizes to steps: `steps.flatMap(s => [...s.fields, ...s.relations])` when `steps` present.
- New `isWizard`, `currentStep` (ref, URL-seeded), `visibleSteps` (filter by `visible_when`), `visibleFields(step)`.
- Effective-required = authored `required` OR `required_when` evaluates true — folded into `validate()` and the Next gate. `validate()` gains a `stepIndex?` param.
- Render: when `isWizard`, stepper header + only `visibleSteps[currentStep]` fields + Back/Next; Next runs `validate(currentStep)`; last step Submit runs full validation over visible steps then the existing create/update path (unchanged).
- `?step=N` URL param: seed from `route.query.step`; on Next/Back `router.replace` with echo-suppression (mirror `useUrlFilterSync`); clamp invalid/out-of-range → first visible step.
- Hidden-branch handling: exclude hidden-step values from the final payload (matches OpenVWR `isFieldEnabled`) so toggled-off branches don't persist stale data.

Alternatives considered:
- *Flat filter-string grammar (AND-only)* — rejected: too limited for OR / cross-field, and the user identified the same condition need across ACL/view features, so a shared richer engine is warranted.
- *Port `internal/predicate` to JS* — rejected: ~1,400 LOC incl. a Lua parser dependency; a second implementation to keep in sync with the Go ACL engine. The parser is the *easy* part; the typed IR/compiler/budgets are the cost and aren't needed client-side. Hand-write a small engine instead, keeping the grammar congruent so server-eval remains a drop-in later.
- *Reuse `sections` for steps* — rejected: sections = stacked-on-one-page; add explicit `steps`.
- *Server-side condition eval* — rejected: conditions must react live as the user types, pre-save.

**Files to modify (this ticket; engine files belong to TKT-BL7XZ):**
- `internal/dataentryconfig/config.go` — `Form`, `FormField`, `FormRelation`; new `FormStep`.
- `internal/dataentryconfig/validate.go` — `validateForms` (steps-xor-flat, per-step reuse, condition presence check).
- `internal/dataentry/api_v1.go` — `handleV1Config` relation-widget loop over `Steps[].Relations`.
- `frontend/src/types/config.ts` — `FormStep`, `visible_when`/`required_when`; retire `sections`.
- `frontend/src/components/forms/DynamicForm.vue` — wizard rendering, stepping, per-step validation, URL-sync, engine wiring.
- Tests: Go `validate_test.go`; e2e wizard spec under `e2e/`.
- Docs: `docs/data-entry.md` Forms section (`steps:` / `visible_when` / `required_when`, pointing at the engine's grammar doc).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- **Authored config (`steps`/`visible_when`/`required_when`):** trusted admin input. Go validates steps-xor-flat and that referenced field properties exist (existing form validation already hard-errors on unknown field properties). Condition-string *grammar* is validated by the engine (TKT-BL7XZ) — parse errors surface at author time where possible; at runtime a bad expression evaluates false + warns, never throws.
- **User-entered field values (browser):** the engine compares values held in `formData`; it does NOT `eval()` — it's a hand-written parser over a closed grammar. Property lookup is prototype-pollution-safe (reject `__proto__`/`constructor`) — an engine-level requirement pinned in TKT-BL7XZ.
- **`step` URL param:** parse to int, clamp to `[0, visibleSteps-1]`; non-numeric/out-of-range → first step.

**Security-Sensitive Operations:**
- Client-side visibility/required is a **UX affordance only, NOT authorization**. The server re-validates every write regardless of what the wizard showed/hid (`_actions`/field affordances + write-time validation unchanged). A crafted request bypassing the wizard faces the same server checks as any single-page submit. Document so `required_when` is never mistaken for a server constraint.
- No file access, crypto, or auth changes.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios (per AC):**
- AC1: Go `validate_test.go` accepts a `steps` form and rejects steps+flat together. e2e: wizard renders ordered titled steps.
- AC2: engine unit tests (TKT-BL7XZ) cover the operators; here e2e: toggle reveals/hides a step + flips a field's required-ness; an OR condition across two fields.
- AC3: e2e — empty required blocks Next with per-field error; fill → advance; Back preserves values; Submit validates all visible steps.
- AC4: e2e — advance, reload, land on same step; invalid `?step=99` → first step.
- AC5: e2e regression — existing single-page forms unchanged; Go: a flat form still validates/serves identically.

**Edge Cases:**
- Condition references unset/empty field → engine treats as empty/nil (defined in TKT-BL7XZ); document.
- A visited step becomes hidden after an earlier answer changes → clamp `currentStep`, skip hidden steps on Next/Back, drop hidden steps from Submit validation, and **exclude hidden-step values from the payload**.
- All steps after current become hidden → Next becomes Submit.
- `visible_when` that hides step 0 → fall through to first visible step.
- Invalid regex in `=~` → engine returns false + warn (no throw).

**Negative Tests:**
- steps + flat fields both set → config load error (Go).
- `visible_when` referencing a nonexistent field property → config load error (Go, existing form-validation path) where the property is a form field; free-form `entity.`/`current_user.` refs are engine-checked, not Go-checked.
- Malformed expression → engine parse error at author time / false+warn at runtime (TKT-BL7XZ).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- *Dependency sequencing:* TKT-CHLAJ needs TKT-BL7XZ first. Mitigation: `depends-on` recorded; implement the engine ticket first (it's independently testable). The engine's API (`evaluate(expr, bindings, functions?)`) is the integration contract.
- *DynamicForm is already large (~1490 lines), near god-object territory.* Mitigation: condition logic lives entirely in the engine module; keep step-state in a small composable if it grows; don't inflate the component's public surface.
- *Hidden-branch payload semantics (drop vs retain)* — chosen to drop; document; revisit if a real case wants retention.
- *Two layout concepts (`sections` latent + new `steps`)* — retire unused `sections` in this PR.
- *Regression to single-page forms* — wizard path is strictly additive behind `steps` presence; e2e regression must stay green.

**Effort:** m (this ticket) — plus m for TKT-BL7XZ.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md — new `steps:`, `visible_when:`, `required_when:` under Forms; note client-side-only evaluation + server still authoritative; link to the engine grammar doc.
- [x] Engine grammar doc — owned by TKT-BL7XZ (grammar reference for authors).
- [ ] docs/metamodel.md — N/A (form config lives in data-entry.yaml).
- [ ] README.md — N/A.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Design review focused on the dependency ticket TKT-BL7XZ (the condition
expression engine), since its grammar/API is the contract this ticket binds to.
7 findings raised (4 significant, 3 minor), all addressed in the TKT-BL7XZ spec:
RR-8VZSP (grammar precedence pinned to Lua 5.1), RR-9IQBT (coercion/equality
table), RR-8GRLD (bad-condition surfacing = dev-console + `rela` CLI lint via Go
predicate parser), RR-P6GVE (host-function registry deferred), RR-YTKIC (`!=`
canonical), RR-7VKNB (compile/eval split + AST memoization), RR-TNMRC (per-node
fail-safe short-circuit). No design changes to this ticket's own approach were
needed. A wizard-specific `/code-review` will run when TKT-CHLAJ enters review.
