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
| `src/components/forms/` | Form widgets: DynamicForm, FieldRenderer, RelationPicker, MarkdownEditor, SidePanel |
| `src/components/lists/` | EntityList, FilterBar, Pagination |
| `src/components/common/` | Sidebar, StatusBar, Badge, Toast, BackButton |
| `src/composables/` | Vue composables: useKeyboardShortcuts, useEvents (SSE), useListKeyboard, useScopeNavigation, useBackTarget |
| `src/styles/` | Shared CSS loaded from `main.ts` (e.g. `back-button.css` for the `.scope-nav-btn` class reused across EntityDetail, CustomView, and standalone BackButton) |
| `src/types/` | TypeScript interfaces for entities, schema, and config |

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
from the host UI it renders inside. `TestAppCSSSource` asserts the name/value
pairs on the Go side. Extending the ramp (`-xs`, `-md`, `-2xl`, `-3xl` are
SPA-only) is safe; renaming or revaluing those four is not.

A handful of off-scale sizes (9/10/15/16/17/20/21/24/48px) are deliberately
left as literals — each is rare and rounding them would change a size the
author picked on purpose.
