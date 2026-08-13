# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
# Development
npm run dev                    # Start Vite dev server on :5173 (proxies /api to :8080)
npm run build                  # Build to ../internal/dataentry/static/v2/

# Lint & Format
npm run lint                   # Run ESLint
npm run lint:fix               # Auto-fix lint issues
npm run format                 # Format with Prettier
npm run format:check           # Check formatting
npm run typecheck              # Run vue-tsc type checking
npm run dupes                  # Check for code duplication (jscpd)

# Unit Tests (Vitest)
npm run test                   # Run tests in watch mode
npm run test:run               # Run tests once
npm run test:run -- src/stores/ui.test.ts  # Run single test file
```

E2E tests live in the top-level `/e2e/` directory and run against the built
`rela-server` binary (which embeds this SPA's production bundle). From the
repo root, use `just e2e` or `cd e2e && npm test`.

## Architecture Overview

Vue 3 frontend for rela data entry application. Communicates with the Go backend (`rela-server`) via REST API.

### Data Flow

```text
Backend API (/api/v1/*)
     ↓
src/api/          → Axios API client layer
     ↓
src/stores/       → Pinia stores (state management)
     ↓
src/views/        → Page components (route targets)
     ↓
src/components/   → Reusable UI components
```

### Package Structure

| Directory | Purpose |
|-----------|---------|
| `src/api/` | Typed API client functions (entities, schema, git, settings, etc.) |
| `src/stores/` | Pinia stores: `schema` (metamodel/config), `entities` (CRUD + cache), `ui` (toasts, sidebar), `git` (status) |
| `src/views/` | Route-level components: Dashboard, List, Form, Entity, Kanban, Graph, Search, Settings |
| `src/widgets/` | The property-widget library + `registry.ts` (the type→widget dispatch). One widget per property type, each rendering both `mode: 'display'` and `mode: 'edit'` |
| `src/components/forms/` | Form scaffolding: DynamicForm, FieldRenderer (resolves via the widget registry), RelationPicker, MarkdownEditor, SidePanel |
| `src/components/lists/` | EntityList, FilterBar, Pagination |
| `src/components/common/` | Sidebar, StatusBar, Badge, Toast, BackButton |
| `src/composables/` | Vue composables: useKeyboardShortcuts, useEvents (SSE), useListKeyboard, useScopeNavigation, useBackTarget |
| `src/styles/` | Shared CSS loaded from `main.ts` (e.g. `back-button.css` for the `.scope-nav-btn` class reused across EntityDetail, CustomView, and standalone BackButton) |
| `src/types/` | TypeScript interfaces for entities, schema, and config |

### Widgets render property values everywhere, including read-only

A property value is rendered by a **widget** on every surface — forms, entity
detail, view sections, list/table cells, and kanban cards. Widgets take a
required `mode: 'display' | 'edit'` (no default, RR-UD1I) and own their
read-only rendering. **Do not add a per-type `v-if` to a view.** A new property
type should need a widget plus a routing entry, never a branch per surface.

There are two routing entry points, and they are not interchangeable:

- `defaultRegistry.resolve(name, propertyDef)` — form side, when you hold the
  real schema entry.
- `defaultRegistry.resolveFromHint(hint)` — view side, when you have wire-level
  field metadata. Build the hint with `viewFieldRoutingHint` (view sections) or
  `densePropertyRoutingHint` (list cells, kanban cards).

**Dense surfaces are not detail views.** `densePropertyRoutingHint` exists
because a table cell and a detail row have genuinely opposite contracts, and
reusing the detail-view routing in a cell has already caused regressions
(TKT-S9C14S):

- **Empty renders blank, never a placeholder.** MultiSelectWidget's em-dash
  (RR-UD2C) is right for a detail row and wrong for a cell — `formatCellValue`
  documents that "blank table cells stay visually quiet". `isDenseEmpty` gates
  this at the cell, before the widget sees the value.
- **`list: true` must not override the type's formatter.** `defaultWidgetFor`
  puts list first because the *edit* side needs a tag picker whatever the type;
  on the display side that erases the formatter (a list-valued rrule rendered as
  an em-dash). Non-enum lists route to preformatted text.
- **`boolean` → text and `file` → text in cells**, deliberately: booleans stay
  Cmd-F searchable, and FileWidget's `<img>` previews would be one network
  request per cell.
- **Resolve per column/field, not per cell** — `resolve()` walks a Map and can
  `console.warn`, which at 200 rows is 200 warnings per render. See
  `PropertyDisplay.vue`'s precomputed `rows` (RR-UD2A) for the pattern.

The hint carries `preformatted`: true for passthrough widgets that render
`String(value)`, false for widgets that own their display formatting. A new
widget defaults to false (raw value), which is correct — the reason to add a
widget is that it renders something text cannot.

### Key Stores

- **schemaStore**: Loads metamodel (entity/relation types) and config (forms, lists, views, navigation) on app mount
- **entitiesStore**: Entity CRUD with 1-minute TTL cache, invalidates on mutations
- **uiStore**: Toast notifications, sidebar collapse state, theme (dark/light)
- **gitStore**: Git status polling for uncommitted changes indicator

### SSE Real-time Updates

`useEvents` composable connects to `/api/v1/_events` SSE endpoint.
On entity changes, it calls `entitiesStore.invalidateAll()` to refresh cached data.

### Routing

Routes use dynamic imports for code splitting. Config-driven IDs (e.g., `/list/:id`, `/form/:id`)
resolve to `data-entry.yaml` configuration from the backend.

## Lint Configuration

ESLint flat config with:

- Vue 3 recommended + TypeScript
- `vue/no-v-html: warn` (XSS risk)
- `max-lines: 500` warning for Vue files (catches god components)
- Relaxed rules for test files (`no-explicit-any`, `no-non-null-assertion` allowed)

## CSS Architecture

Global styles in `App.vue` use CSS custom properties for theming:

- Light/dark mode via `:root.dark` class
- Shared utility classes: `.btn`, `.btn-primary`, `.modal`, `.page-header`
- Components use scoped styles with BEM-like naming

### Design tokens: two files, different contracts

| File | Holds | Contract |
|------|-------|----------|
| `src/styles/tokens.css` | **Colour only** | Copied byte-identically into the Go binary (`internal/dataentry/apps_tokens.css`) and served to custom apps as `_rela.css`. `TestAppTokensCSSInSyncWithFrontend` fails on drift. |
| `src/styles/scales.css` | Spacing, radius, typography, elevation | SPA-only, except the four `--font-size-*` steps noted below. |

Both are imported from `main.ts`. Keep them separate: `tokens.css` documents
itself as theme-tokens-only, so dimension scales do **not** belong there.

**Use the scales instead of hardcoding.** Prefer `var(--space-sm)`,
`var(--radius-md)`, `var(--font-size-base)`, `var(--shadow-sm)` over raw px.
Before this existed the SPA had 10 distinct border-radius values and 17 font
sizes chosen per-component, so nothing lined up between components.

**`--font-size-{sm,base,lg,xl}` is a frozen cross-boundary contract.** The same
four names and values are declared in Go, in `appTypographyCSS`
(`internal/dataentry/apps_css.go`), and served to every custom app — TKT-PF4E6S
froze them. Changing one side without the other makes an app's typography drift
from the host UI it renders inside.

`TestFrozenTypographyContractMatchesSPA` reads **both files off disk and
compares them to each other**, so a revalue on either side fails. Don't
"simplify" it into asserting Go against literals in the test file: that only
proves Go is self-consistent and cannot see a change to `scales.css` at all.

SPA-only sizes stay **outside** the ramp's naming. `--font-size-dense` (13px)
is a role name, not a `-md` step, precisely so nobody reads it as part of the
contract and copies it into a custom app — where it is undefined and would
silently fall back to the inherited size. `TestAppCSSSource` pins the negative
side (apps must not define it).

**Token values are chosen to be value-preserving,** not to be the shortest
ramp: `--radius-lg` is 8px because 8px is the card radius across the app, so
adopting the token doesn't quietly flatten every card. When you add a token,
prefer an existing value in the tree over a rounder number.

Off-scale sizes (9/10/15/16/17/20/21/24/48px) are deliberately left as
literals — each is rare and rounding them would change a size the author picked
on purpose.

### Property layout: one grid, one stylesheet

`src/styles/properties-list.css` owns `.properties-list` / `.property-item`,
shared by `SectionEditForm`, `PropertyDisplay` and `SidePanel`. **Do not
redefine those classes in a component `<style>` block** — they used to be
declared three times with three different `min-width` values, scoped so they
could never actually share, which is why the detail page didn't align with
itself.

Layout is a **12-column grid** (TKT-5V8704). Every item spans all 12 unless the
view/form config authors a `span:`, which arrives as a `--field-span` custom
property. The default lives in exactly one place: the `var(--field-span, 12)`
fallback. Don't emit a literal `12` from Go or a component — that creates a
second copy of the default.

`utils/fieldSpan.ts` is the only place a span becomes CSS. It clamps to 1-12
and returns `undefined` for anything else, so config never reaches a stylesheet
unvalidated. The server already rejects bad spans at load with a specific
error; the frontend clamp is defence-in-depth for hand-crafted responses.

Careful with grid children: in `DynamicForm`, `.form-fields > *` and
`.form-field` have equal specificity, so both read `var(--field-span, 12)`. A
bare `span 12` on the first rule would win on source order and silently swallow
every authored span — which is exactly what happened the first time.
