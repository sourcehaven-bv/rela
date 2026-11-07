---
id: PLAN-4PR1Y7K
type: planning-checklist
title: 'Planning: World switcher: labels and opt-in placement; hidden worlds; per-record toggle'
started: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** `WorldSwitcher.vue` renders in the sidebar header whenever the
schema declares two or more worlds. It shows raw world names in a labelled
dropdown ("World [werk ⌄]"), on every page, and offers feed worlds that no
reader picks. Operators cannot label, hide or place it. On a record page the
face switch (`FaceMenu`) is a button that collapses into "⋯" when the header
is narrow, so "Concept / Vastgesteld" is hard to find.

**Scope — IS in scope:**

- `label:` on a world in `schema.yaml`, served on `/_schema` worlds, shown in
  the switcher and substituted for `{world}` in world messages.
- `hidden: true` on a world: left out of the switcher, still selectable through
  `?world=` and the API. The default world cannot be hidden (load error).
- `app.world_switcher: header | none` in `data-entry.yaml`, default `header`
  (today's behaviour). Validated at load. Editable in Configure, like
  `app.language`.
- The switcher uses `RlSegmentedControl` for two or three visible worlds and
  keeps the select for more. A hidden world that is the current one is still
  listed, so the control never shows a world that is not on screen.
- A record face toggle: when an entity has two or three faces in total, the
  detail header shows a `RlSegmentedControl` with the served face selected.
  It stays visible in the compact header, and the per-face "View X" entries
  are then left out of the actions and the "⋯" menu. With more faces the
  current `FaceMenu` stays.

**IS NOT in scope:**

- `world:` on a list or navigation entry (follow-up ticket).
- A `detail` placement for the global switcher: the record toggle covers it.
- Changing world resolution, ACL or the default world.

**Acceptance Criteria:**

1. A world with `label: Gepubliceerd` shows "Gepubliceerd" in the switcher, and
   `{world}` in its messages renders as the label. *Test:* schemaworlds Go
   test for `label` on the wire; vitest for `worldLabel` and the switcher.
2. A world without a label shows its name, as today. *Test:* vitest.
3. `hidden: true` keeps a world out of the switcher; with fewer than two
   visible worlds the switcher does not render. `?world=<hidden>` still
   resolves. *Test:* vitest; Go test that `hidden` is on the wire.
4. `hidden: true` on the default world fails to load with a message naming
   the world. *Test:* metamodel loader test.
5. `app.world_switcher: none` renders no switcher; an unknown value fails to
   load. *Test:* dataentryconfig validate test; vitest.
6. Two or three visible worlds render a segmented control; four render the
   select. *Test:* vitest.
7. A record with two faces shows the toggle with the served face checked;
   choosing the other navigates to its address and keeps the query. The
   "View X" entries are gone from the actions. *Test:* vitest on the toggle;
   e2e against a faced example.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the shape follows existing world keys; no open design question needs a survey)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-house UI pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `RlSegmentedControl` (`frontend/packages/rela-components/src/components/data/RlSegmentedControl.vue`): radio group with roving tabindex; used in `NavGroupEditor.vue` and `NewPileDialog.vue`.
- Face labels already exist (`faces.<name>.label`, `schemaStore.faceLabel`); world banner and messages show the same "operator words or nothing" rule (TKT-5SZG2L).
- `app.*` keys reach the SPA through `v1.AppConfig` (`internal/dataentry/api_v1.go` config handler).
- Configure classification: `worlds.**` is already locked in `internal/configedit/allowlist.go`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `metamodel.WorldDef`: add `Label string` and `Hidden bool` (also in the
   UnmarshalYAML shadow). `validateWorlds`: refuse `hidden` on the effective
   default world.
2. `v1.World`: add `Label` and `Hidden` (`omitempty`); `schemaWorlds` copies
   them.
3. `dataentryconfig.AppConfig`: add `WorldSwitcher string`
   (`world_switcher`); `validateApp` accepts `""`, `header`, `none`.
   `v1.AppConfig.WorldSwitcher` is filled in the config handler. Allowlist:
   `app.world_switcher` editable.
4. SPA: `WorldInfo.label/hidden`, `AppConfig.world_switcher` (the server
   sends `header` when the key is unset, so the SPA has no default of its
   own). Schema store gets `worldLabel(name)`. `{world}` call sites
   (EntityDetail textVars, EntityList, KanbanView, WorldBadge, DynamicForm,
   HistoryView) pass `worldLabel(world)`. MCP `list_worlds` reports `label`
   and `hidden` and does not filter.
5. `RlSegmentedControl` gains per-option `as`/`attrs`, following
   `RlViewTabs`. When the options are links, the group renders them as links
   with `aria-current`, each a normal Tab stop, with no arrow-key selection.
   Navigation then never fires per key, and cmd-click opens a new tab.
6. `WorldSwitcher.vue`: no visible label; the accessible name is a neutral
   i18n string. The control type is chosen from the visible count
   (segmented for 2–3, select for more); the current world is then added if
   hidden, in schema order. Renders nothing when `world_switcher === 'none'`
   or fewer than two options are readable. Keeps the `world-switcher` class
   and `data-testid`.
7. New `FaceToggle.vue` in `components/entity`: the served face plus
   `_faces`. Renders when the total is 2 or 3 and every face has a stored
   name. Each segment is a link to the face address; `?world=` is set to
   `worldForFace(type, face)` when a world heads that face, so the sidebar
   and the record agree. With no such world the current world is kept, as
   `FaceMenu` does. EntityDetail renders it in `header-info` and skips the
   face entries in `actions` and `FaceMenu` when it renders.
8. E2E: `e2e/pages/faces.page.ts` clicks world and face links by label and
   asserts `aria-current`.

**Alternatives rejected:**

- Hiding through CSS (`custom.css`): no contract (FEAT-MD4N6Z tier 3).
- Putting `label`/`hidden` in `data-entry.yaml`: worlds, their banners and
  messages are declared in `schema.yaml`; splitting their text across files
  would be harder to maintain.
- Default `world_switcher: none`: would silently remove a control existing
  projects rely on.

**Files to modify:**

- `internal/metamodel/types.go`, `internal/metamodel/loader.go` (+ tests)
- `internal/apiwire/v1/responses.go`, `internal/dataentry/schemaworlds.go`, `internal/dataentry/api_v1.go` (+ tests)
- `internal/dataentryconfig/config.go`, `internal/dataentryconfig/validate.go` (+ tests)
- `internal/configedit/allowlist.go`
- `frontend/src/types/schema.ts`, `frontend/src/types/config.ts`, `frontend/src/stores/schema.ts`
- `frontend/src/components/common/WorldSwitcher.vue` (+ test)
- `frontend/src/components/entity/FaceToggle.vue` (new, + test), `EntityDetail.vue`
- `frontend/src/components/lists/EntityList.vue`, `frontend/src/views/KanbanView.vue`, `frontend/src/components/ui/WorldBadge.vue`
- `docs/metamodel.md`, `docs/content-states.md`, `docs/data-entry.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `worlds.<name>.label`: operator text, rendered as text (Vue interpolation,
  no `v-html`). No placeholders are substituted inside a label.
- `worlds.<name>.hidden`: boolean, YAML-typed.
- `app.world_switcher`: allowlist `""`, `header`, `none`; anything else is a
  load error.

**Security-Sensitive Operations:**

- None. `hidden` is presentation only: it is not a read boundary, and the
  world's grant is re-checked per request as before. Docs say so, so nobody
  builds on it as concealment (config is not a secret).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria; each names its test.

**Edge Cases:**

- Current world is hidden (`?world=publiek`): it is listed so the control
  shows where the reader is.
- Current world not readable: kept disabled as today.
- A schema with no worlds: no switcher, unchanged.
- An entity of a faceless type: no toggle.
- A face without a label: the face name is shown.
- A label with braces: rendered literally, never re-scanned.

**Negative Tests:**

- `hidden: true` on the default world → load error.
- `app.world_switcher: sidebar` → load error naming the key and the values.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- The sidebar header is narrow; three segments may not fit. Mitigation: `sm`
  size, ellipsis on labels, and the select for four or more. Checked in the
  browser.
- E2E tests that select `world-switcher` by `data-testid`: keep the test id on
  the select and add it to the segmented control's root.
- Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/metamodel.md - `label`, `hidden` on worlds
- [x] docs/content-states.md - the switcher and the record toggle
- [x] docs/data-entry.md - `app.world_switcher`

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 14 findings (5 significant, 7 minor, 2 nit),
recorded as review responses on TKT-H0TFDWU. All are addressed in the
approach above: world-aware face links, link-mode segments, no "Wereld"
label, the e2e page object, the missed `{world}` sites, the visible-count
rule, no control without a choice, the default-world check, face labels,
MCP labels, the CSS hook, the unknown-key limit in the docs, server-side
normalisation and blank labels.
