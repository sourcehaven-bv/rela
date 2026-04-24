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
