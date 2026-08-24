---
id: PLAN-DMQFRJ
type: planning-checklist
title: 'Planning: Widget override for view section fields (`widget:` on ViewSectionField)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN — an optional `widget:` key on a view section field, letting config choose
which registered widget renders a property instead of the type-derived default:

```yaml
sections:
  - heading: Today
    display: properties
    render: input
    fields:
      - property: done
        widget: checkbox
```

OUT:
- New widget types. Only selection among already-registered ones.
- The `render:` axis (TKT-HOIX1, merged #1364).
- The table-cell path (`ViewCell.Widget`) — LIVE and type-derived (finding 1),
  excluded as a different surface, not as dead code.
- Closing the pre-existing `FormField.Widget` validation gap (finding 4) —
  separate ticket, it is a breaking change on its own.
- Any override on the hint arm (finding 3) — unvalidatable there, so ignored
  and warned, not deferred.
- `_attachments` on cards/list rows (RR-NGY84F) — follow-up ticket; this ticket
  rejects `widget: file` outside a `properties` section instead.

**Acceptance Criteria:**

1. `widget: checkbox` on a boolean property in a `render: input` section renders
   a clickable checkbox rather than the type default.
   → e2e: click it, assert the PATCH lands and the value flips.
2. Omitting `widget:` reproduces today's selection byte-for-byte.
   → **frontend** unit (RR-693NL9): the section path's type→widget selection is
   `defaultWidgetFor` (`registry.ts:18`), not Go — a Go test would pin a mapping
   this code path never consults. Table test over every property type asserting
   `widget: undefined` → `defaultWidgetFor(propertyDef)`.
3. An unregistered widget name is a config-load error naming the valid set.
   → unit: `ValidateConfig` returns an error containing the name + valid list.
4. A widget incompatible with the property's type is a config-load error.
   → unit: `widget: checkbox` on a `date` property errors.
5. A `widget:` on a property absent from `schema.yaml` loads, is ignored at
   render, and emits a warning naming section + field index.
   → unit: `CollectConfigWarnings` contains it; no error returned.
   → plus a **hint-arm negative test** (RR-2GBB0V) asserting the override is
   provably DROPPED, so a later refactor that plumbs `widget` into the shared
   half of the union — where both arms would see it — fails loudly.
6. `widget: file` outside a `properties`-display section is a config-load error
   (RR-NGY84F).
   → unit: a `cards` section with `widget: file` errors.

**Not an acceptance criterion** (RR-66MT0D): a warning for `widget:` on a
state-machine field. Machine-ness is runtime, per-entity and per-principal
(`computeTransitions`, `affordances.go:1051`, gated on a `TransitionResolver`
type-assertion); `CollectConfigWarnings` has neither a resolver nor an entity,
so the warning is unbuildable. Documented in `docs/data-entry.md` instead.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small, single-axis change with a directly analogous
predecessor (TKT-HOIX1) landed two commits ago.

**Existing Solutions:**

No library question — this is config plumbing through an existing registry.

Verified against develop @ 3a735757. **The ticket as written was stale in four
material ways**; each was checked in code, not assumed:

1. **`ViewCell.Widget` is LIVE** — my original claim that it was dead was
   WRONG (RR-YTC4W5). `cell.Widget = resolveWidget(pd, s.Meta)`
   (`sections.go:296`) populates it; `v1.ViewCell(cell)`
   (`views_handler.go:611,629`) ships it on every table cell. My grep pattern
   `'Widget:'` structurally could not match a field assignment. Tables stay out
   of scope on **surface** grounds (different renderer, no inline edit), not
   because the field is unused. **Corollary that matters for the drift
   decision:** a second server-side resolver already exists
   (`ResolveWidgetFromType`, `schema_output.go:117`) claiming in its godoc to be
   the single source of truth — already false, and already drifted from
   `defaultWidgetFor` on `file`, `list`, and `values`.

2. **`registry.ts` needs no change.** `defaultRegistry.resolve(name, propertyDef)`
   (`registry.ts:50-70`) *already* prefers an explicit name, already falls back to
   the type default on an unknown name, and already warns. The sole gap is the
   only view-side caller — `SectionEditForm.vue:147-152` hardcodes
   `resolve(undefined, field.propertyDef)`.

3. **The hint arm must ignore the override — corrected rationale**
   (RR-2GBB0V). An unschema'd field is NOT display-only: `isFieldWritable`
   (`affordances.ts:12-18`) returns `true` for a *missing* verdict — documented,
   deliberate — and neither `widgetRows` (`SectionEditForm.vue:164`) nor the
   edit branch (`:288`) inspects `field.kind`. A hint-arm field on
   `render: input` therefore renders an EDITABLE widget and PATCHes. The real
   constraint is that the arm has no `PropertyDef` (`:298`, `:314` pass
   `undefined`), so AC4's compatibility rule is **unenforceable** there;
   honouring `widget:` would ship an unvalidated widget into a live edit
   control (`checkbox` PATCHing a boolean into a string property). Ignore, warn,
   and TEST that it is provably dropped.

4. **`FormField.Widget` is validated nowhere** — `grep 'f.Widget'` in
   `validate.go` returns nothing, so a typo'd form widget silently falls back
   today. Not inherited by the new field; not fixed here either.

Prior art followed: `Render` (TKT-HOIX1) for the plumbing shape;
`validRelationWidgets` (`validate.go:159-164`) for the allowlist-map validation
shape; `inertSectionRenderWarnings` for the warn-don't-error shape.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Mirror the `Render` thread end to end, with one deliberate difference: **no
section-level inheritance** — field-level only. `render:` can inherit because
both of its values are valid for every field; a section-level `widget:` would be
a config-load ERROR on every field whose type doesn't match, turning one authored
line into N errors the operator must override back field-by-field. See
Alternatives for the full argument (RR-9G51IS). This is the main structural
departure from TKT-HOIX1 and is intentional.

Compatibility rule for AC4 — a widget is valid for a property when the pair
appears in the table below. **Do NOT derive this from
`Metamodel.ResolveWidgetFromType`** (RR-Z0GGTO): that function has no `file`
case, so deriving would reject `widget: file` on a `file` property, which is
legal. The table is authored as its own Go map literal, mirroring the
registry's `supportedPropertyTypes` (`registry.ts:105-117`):

| widget         | accepts                                  |
| -------------- | ---------------------------------------- |
| `text`         | string                                   |
| `textarea`     | string                                    |
| `number`       | integer                                  |
| `checkbox`     | boolean                                  |
| `date`         | date                                     |
| `datetime`     | datetime                                 |
| `select`       | enum, string, custom enum types          |
| `multi-select` | enum, string (list properties)           |
| `rrule`        | rrule                                    |
| `file`         | file                                     |

`textarea` on a string is the motivating non-default pair (long text), as is
`select` on a string with a custom type. The table is the whole point of the
feature — without it the only legal value is the default.

**Drift guard (RR-Z0GGTO).** The table is duplicated across Go and TS, so both
sides assert against one shared fixture: a Vitest test asserting the registry's
`supportedPropertyTypes` matches it, and a Go test asserting the validator's map
matches it. Two small tests that FAIL on drift, rather than a comment that
documents it. This is not hypothetical — the two existing resolvers have already
drifted on `file`.

**`widget: file` is rejected outside a `properties` section** (RR-NGY84F): only
the entry site passes `:attachments` (`EntityDetail.vue:893`); the cards
(`:982-992`) and list (`:1038-1048`) sites do not, so a forced `FileWidget` in a
row would render with `attachments === undefined`. Plumbing `_attachments` onto
`ViewEntity` is the fuller fix and is a follow-up ticket.

**Alternatives rejected:**
- *Section-level `widget:`* — rejected for two reasons stronger than the
  "meaningless across mixed types" one first given (RR-9G51IS, which a
  five-string-properties-all-wanting-textarea section defeats). First, `render:`
  can inherit because BOTH its values are valid for every field; a section-level
  `widget:` is a config-load ERROR on every field of a non-matching type, so one
  authored line becomes N errors the operator must undo field-by-field —
  anti-inheritance. Second, validating it needs each field's property type,
  i.e. exactly the metamodel-dependent context RR-4ICH8M moved the field-level
  check OUT of.
- *Extending `resolveFromHint` to take a name* — would let config target a path
  with no PropertyDef to validate against and no save path. Finding 3.
- *Name-only validation, skip the type check* — cheap, but `checkbox` on a
  `date` yields a control that cannot represent the value. Config errors should
  surface at load, matching the rest of `validate.go`.
- *Reusing `FormField.Widget`'s (absent) validation* — inherits a known gap.

**Files to modify:**

Backend:
- `internal/dataentryconfig/config.go` — `Widget string` on `ViewSectionField`.
- `internal/dataentryconfig/validate.go` — `validateSectionFieldWidget`; called
  **outside** the source-resolution guards (the RR-4ICH8M lesson from HOIX1:
  a closed enum needs no metamodel knowledge to reject). Plus the
  undeclared-property warning in `CollectConfigWarnings`.
- `internal/dataentry/sections.go` — `Widget` on `SectionFieldData`; carried in
  `buildSectionFieldData`.
- `internal/apiwire/v1/responses.go` — `Widget` on `SectionField`, **same
  position/type** (the unnamed conversion `v1.SectionField(f)` is the
  compile-checked sync).

Frontend:
- `frontend/src/api/views.ts` — `widget?: string` on `ViewSectionField`.
- `frontend/src/components/entity/sectionEditFields.ts` — carry onto
  `SectionEditField`.
- `frontend/src/components/forms/SectionEditForm.vue` — `widgetRows` passes
  `field.widget` to `resolve` on the schema arm.

Docs: `docs/data-entry.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

One input: the `widget:` string in operator-authored `data-entry.yaml`. Validated
at config load against a **closed allowlist map** (the `validRelationWidgets`
shape), never a blocklist. Invalid → config-load error, server does not start
with a bad config. The value never reaches a filesystem path, a command, or a
query — it selects among components already compiled into the SPA bundle. An
unknown name reaching the client anyway (shape drift) is handled defensively by
`registry.resolve`, which falls back to the type default and warns.

**Security-Sensitive Operations:**

None. Explicitly **not** an ACL surface: widget selection is presentation, and
this ticket does not touch the `render === 'input' && isFieldWritable(verdict)`
conjunction that gates editability. A `widget:` on a read-only field still
renders display — the widget choice decides *which* component, never *whether*
it is writable. Guard test asserts exactly this.

Per CLAUDE.md "the configuration is not a secret": widget names are operator-
authored config, so the error message naming the valid set is correct and
useful, not a leak.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Where |
| -- | ---- | ----- |
| 1 | checkbox click PATCHes and flips | e2e, extend `view-section-render-mode.spec.ts` |
| 2 | every type, `Widget: ""` == current default | unit, `internal/dataentry` table test |
| 3 | unknown name errors, lists valid set | unit, `dataentryconfig` |
| 4 | `checkbox` on `date` errors | unit, `dataentryconfig` table over pairs |
| 5 | undeclared property warns, does not error | unit, `CollectConfigWarnings` |
| 6 | `widget: file` in a cards section errors | unit, `dataentryconfig` |
| — | wire struct sync | existing unnamed-conversion compile check |
| — | ACL conjunction unaffected | unit, `SectionEditForm` component test |
| — | hint arm drops the override | unit, `SectionEditForm` component test |
| — | Go table == TS `supportedPropertyTypes` | paired Go + Vitest fixture test |
| — | machine field: inert on input, live on display | unit, component test |
| — | row `values`-mirror interaction (RR-FC1C) | unit, row-level test |

AC1's e2e is viable without new fixtures — a boolean property exists at
`e2e/tests/fixtures.ts:847`.

**Edge Cases:**

- `widget: ""` — absent, not a value. Falls through to type default (AC2).
- Whitespace-only (`"  "`) — `resolve` already `.trim()`s to undefined
  client-side; server must reject rather than let the two disagree.
- Case (`Checkbox`) — reject. Widget names are lowercase throughout; a silent
  case-fold would make config non-canonical.
- `widget:` on a `list: true` property — only `multi-select` legal.
- `widget:` on a state-machine field — **two-axis, not inert** (RR-66MT0D).
  The guard is `transitions !== undefined && render === 'input'`
  (`SectionEditForm.vue:279`), so StatusControl wins ONLY on `render: input`; on
  `render: display` a machine field deliberately falls through to the display
  arm (`:273-277`) and DOES use the overridden widget. Cannot be warned about at
  config load (machine-ness is runtime) — document both halves, test both.
- `widget:` in a `display: table` / `content` section — already inert per
  HOIX1's `sectionDisplayModesRenderingFields`; extend that warning to cover it.
- `widget: file` on a `file` property — legal in a `properties` section (the
  entry site passes `:attachments`), REJECTED elsewhere: cards/list row sites
  pass no attachments (RR-NGY84F).

**Negative Tests:**

Config load must **fail** (not warn) for: unknown name, known name incompatible
with the declared type, whitespace-only, non-lowercase. Each error names the
view, section index, field index, the bad value, and the valid set — the
`section[i] field[j]` precision RR-1SNYI1 asked for.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Server/client compatibility tables drift* — **no longer accepted; solved**
  (RR-Z0GGTO). The original plan proposed accepting the risk on the grounds that
  two copies of ten entries didn't justify machinery. That arithmetic was wrong:
  a THIRD copy already exists (`ResolveWidgetFromType`) and the existing two have
  ALREADY drifted on `file`, `list`, and `values`. Mitigation is now a paired
  Go + Vitest fixture test that fails CI on divergence — about the cost of the
  comment originally proposed. The rejected "derive from `ResolveWidgetFromType`"
  idea would itself have imported the `file` bug.
- *Over-strict validation breaks a working config* — new key, no existing
  configs use it. Zero real exposure.
- *Scope creep into the dead `ViewCell.Widget`* — explicitly out of scope;
  noted in the ticket so the next reader doesn't assume it was missed.
- *`textarea` display mode* — RETIRED. Verified: `TextareaWidget.vue:19` renders
  `<span class="display-value">` in display mode, identical to `TextWidget`. A
  `textarea` override is display-safe and differs only in edit mode.
- *Vue mount sites are not unified the way the Go construction sites are* — the
  Go side funnels through `buildSectionFieldData` (godoc at `sections.go:73-79`
  warns precisely about a field wired into one site and dropped from the other),
  but the three `SectionEditForm` mount sites in `EntityDetail.vue` are separate
  literals and already diverge on `:attachments`. Out of scope to fix; noted so
  the next field added here doesn't repeat RR-NGY84F.

**Effort:** **m** — revised up from `s` (RR-9G51IS). The original estimate
counted only the plumbing and omitted the drift-guard fixture tests, the
`widget: file` section-mode rejection, the two-axis StatusControl handling, the
hint-arm negative test, and correcting the ticket's own false premises.
TKT-HOIX1 was likewise scoped as a single-axis change and produced 60 files.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — extend the TKT-HOIX1 "Field Render Modes" section
  with a sibling on `widget:`: the compatibility table, the interaction with
  `render:` (a widget you cannot click is just an icon), and the field-level-only
  rule with its rationale.
- [x] ~~docs/metamodel.md~~ (N/A: no metamodel change; the type→widget default
  it documents is unchanged)
- [x] ~~docs/cli-reference.md~~ (N/A: no command change)
- [x] ~~CLAUDE.md~~ (N/A: no new convention — follows the existing one)
- [x] ~~README.md~~ (N/A: not project-level)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| ID | Severity | Finding | Status |
| -- | -------- | ------- | ------ |
| RR-YTC4W5 | critical | `ViewCell.Widget` is live, not dead — finding 1 was false | addressed |
| RR-2GBB0V | critical | Hint-arm rationale false; unschema'd fields ARE editable | addressed |
| RR-Z0GGTO | significant | Don't accept table drift — resolvers already disagree on `file` | addressed |
| RR-NGY84F | significant | cards/list rows pass no `attachments`; `widget: file` breaks | addressed |
| RR-66MT0D | significant | StatusControl warning unbuildable; interaction is two-axis | addressed |
| RR-693NL9 | minor | AC2's test was specced in the wrong language | addressed |
| RR-9G51IS | minor | Inheritance rationale weak; effort under-estimated | addressed |

Two of my own findings were wrong and are corrected above with the verification
that overturned them. No open critical or significant findings.
