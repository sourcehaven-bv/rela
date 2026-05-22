---
id: PLAN-B0CI
type: planning-checklist
title: 'Planning: Hide / disable write affordances when the server is read-only (or when an ACL denies the type)'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Revision history**
>
> - v1: initial plan, "inventory awaiting survey" placeholder.
> - **v2 (current)**: folded in (a) full SPA write-affordance survey — 31 sites
>   across 9 component areas plus 5 keyboard shortcuts; (b) go-architect audit
>   — `Deps` struct, explicit Declarative case, value vs pointer; (c) cranky
>   plan review — auto-save flip semantics, unsaved-work safety, dedicated ACL
>   store, ESLint enforcement, e2e fixture extension, FOUC prevention,
>   keyboard-handler gating, SSE-reload hook. Sections marked **(v2)** are new
>   or substantively rewritten.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

Two coupled deliverables:

### In scope (v2)

1. **Backend (`internal/dataentry`).** Add `acl: { mode: "open" | "read-only" }` to the `/api/v1/_config` response, derived via type assertion against `acl.NopACL`, `acl.ReadOnlyACL`, and explicitly against `*acl.Declarative` (the third case returns `"open"` for v0; v1 will return `"policy"` when `writes_allowed_for` lands).
2. **Convert `dataentry.NewApp` to a `Deps` struct.** Today's signature already takes 6 positional collaborators; ACL makes 7. The `entitymanager.Deps` pattern is the established way to grow this.
3. **Frontend.** New dedicated **`aclStore`** (Pinia, separate from `schemaStore`); `useACL()` composable on top; persistent `ReadOnlyBanner` in `App.vue`; per-affordance gating across the 31 sites the survey enumerated; ESLint rule + `data-testid` enforcement so new write affordances can't slip the gate.
4. **`useAutoSave` flip safety.** When `isReadOnly` becomes true, clear timers, drop pending, abort in-flight, refetch, repopulate. No error status for the abort.
5. **SSE-triggered ACL reload.** Hook into `useEvents`' existing `refresh` event so live operator-toggles surface in the SPA without a manual reload.
6. **E2E fixture extension.** `e2e/tests/fixtures.ts` `spawnServer` accepts extra args; new `readOnlyServerUrl` fixture; one Playwright test validates the gate end-to-end.

### Out of scope (deferred to follow-ups)

- Per-entity-type / per-property gating from `writes_allowed_for` — v1 with Declarative populating per-principal.
- Per-principal customization — needs principal resolution on the read path.
- MCP transport intersection — TKT-G3PPD.
- SPA error-toast handling of structured 403 — TKT-JZRNF.
- `policy` mode UX behavior — emit value but treat as open until v1 plumbs `writes_allowed_for`.
- **Consumer-side `dataentry.AppServices` interface** (architect-audit finding) — would let `cmd/rela-server` pass `svc` directly rather than threading individual collaborators. Worth doing eventually for parity with `mcp.Services`; not blocking this PR — tracked as a separate ticket I'll file alongside.
- Action metadata for `read_only_safe` flag per action — v0 hides all action buttons in read-only mode because the config has no read/write metadata. v1 can add the flag.

### Acceptance criteria (v2)

#### Backend

1. **AC1 — `acl.mode` present and stable.** `GET /api/v1/_config` includes `acl.mode` whose value is exactly one of `"open" | "read-only"`. (v0 never emits `"policy"`; reserved but not implemented.) NopACL → `"open"`; ReadOnlyACL → `"read-only"`; `*acl.Declarative` → `"open"` (with a clear v0/v1 doc comment); any unknown ACL impl → `"open"` + `slog.Warn` naming the type.
   - **Test:** `internal/dataentry/api_v1_config_acl_test.go::TestConfig_ACLMode_*` — four cases: NopACL, ReadOnlyACL, *Declarative, fake-ACL. Each via httptest. Plus `TestACLMode_UnknownImpl_LogsWarning`.

2. **AC2 — Additive, no breaks.** Existing `_config` consumers (snapshot tests, the pre-this-PR SPA, external API clients) keep working. The `acl` field is the only addition; no renames or removals.
   - **Test:** existing `api_v1_test.go` assertions pass unchanged; add one assertion that `config.acl.mode != ""`.

3. **AC3 — `aclMode()` lives in dataentry.** The helper is package-private at `internal/dataentry`. Strings are dataentry's wire vocabulary; the ACL package stays free of SPA concerns. Compile-time guard: `var _ = aclMode(acl.NopACL{})` in the test file pins the type-assertion contract.
   - **Test:** unit tests on the helper directly + the compile-time guard.

4. **AC4 — `dataentry.NewApp` takes a `Deps` struct.** Construction goes from `NewApp(fs, paths, meta, store, em, searcher) → NewApp(Deps{...})`. All required collaborators nil-checked. `Deps.ACL` is required (NopACL is the explicit opt-out). `cmd/rela-server/main.go` and `cmd/rela-desktop/main.go` updated to call the new shape.
   - **Test:** unit tests on `NewApp(Deps{})` rejecting nil for each required field; existing tests in `internal/dataentry/*_test.go` updated to use the new struct.

#### Frontend — store + composable

5. **AC5 — Dedicated `aclStore`.** New Pinia store `frontend/src/stores/acl.ts` holding `{ mode, loaded, reload() }`. Populated by a dedicated `getACLConfig()` call (which can share the `_config` fetch under the hood with `schemaStore` to avoid two round trips) or by extracting the `acl` field from the existing `getConfig()` call. Separation of concerns: schema is what's editable; ACL is server policy. Different reload triggers (see AC6).
   - **Test:** `frontend/src/stores/acl.test.ts` — initial state, load, reload, reset.

6. **AC6 — `useACL()` composable.** Returns `{ mode, isReadOnly, isOpen, ready }` reactive refs. `ready` is `true` once `aclStore.loaded === true`. **`isReadOnly` defaults to `true` until ready** (see AC7 FOUC prevention). `mode` defaults to `"read-only"` until loaded, then resolves.
   - **Test:** `frontend/src/composables/useACL.test.ts` — default, transition to open, transition to read-only, store absent (backwards-compat).

7. **AC7 — FOUC prevention.** No write affordance renders until `useACL().ready === true`. In open-mode deployments this is a tiny invisible flash (App.vue already awaits store load before mounting most views — verified during planning). In read-only deployments this prevents the user from briefly seeing "+ Create" buttons that then vanish. Implementation: components consult `isReadOnly` which is `true` while not-yet-loaded; only flips after the fetch.
   - **Test:** App.vue snapshot during a delayed `aclStore.load()` shows no write buttons mid-load.

8. **AC8 — SSE-triggered ACL reload.** `useEvents.ts` extended: on the existing `refresh` event, additionally call `aclStore.reload()`. When the server restarts in read-only mode, the SPA picks up the new state on the next SSE tick (no full page reload required).
   - **Test:** `frontend/src/composables/useEvents.test.ts` — fake SSE event triggers `aclStore.reload` spy.

#### Frontend — banner

9. **AC9 — Banner.** New component `frontend/src/components/common/ReadOnlyBanner.vue`. Persistent ribbon, top of the app shell, non-dismissible. `role="status"`, `aria-live="polite"`, WCAG AA contrast (calm color — not error red, not warning amber). Copy: "Read-only mode — writes are disabled on this server." Mounted from `App.vue` when `isReadOnly && ready`.
   - **Test:** `App.test.ts` snapshots in both modes; a11y attributes asserted; copy pinned via a constant referenced from `docs/security.md`.

#### Frontend — affordance gating

10. **AC10 — All 31 write affordances gated.** The survey enumerated the following groups; each must be hidden (not just disabled) when `isReadOnly`. Per-affordance file/line references in the **Files to modify** section.
    - **EntityList**: header "+ New" + `n` shortcut, row delete + `Del`/`Backspace`, bulk action row + checkboxes + single-letter shortcuts, `e` edit shortcut.
    - **EntityDetail**: desktop+mobile Edit, desktop+mobile Delete, command buttons + overflow menu, content checkboxes, per-row Edit pencils.
    - **DynamicForm**: Save/Create + Cmd+Enter, autosave (suppressed via `useAutoSave({enabled})`), template selector, ID controls, relation widgets (`RelationCards`, `RelationPicker`, `InlineCreateModal`, `SidePanel`).
    - **KanbanView**: header "+ New", drag-and-drop.
    - **SettingsView**: user-defaults form Save, palette Save, logo upload/remove, theme package Install, git commit submit.
    - **ConflictsView**: Apply Resolution.
    - **StatusBar**: git sync click.
    - **Sidebar**: action items in nav.
    - **CommandModal**: defensive guard in `runCommand`.

11. **AC11 — Keyboard handlers also gated.** All keyboard handlers that trigger writes refuse to act in read-only mode. Specifically: `n` (create), `e` (edit), `Del`/`Backspace` (delete), single-letter action shortcuts, `Cmd/Ctrl+Enter` (form submit). Implementation: each handler calls `useACL()` and short-circuits.
    - **Test:** unit tests on `useListKeyboard`, `useListActions`, `DynamicForm`'s `handleKeydown` — keystroke under read-only triggers no action.

12. **AC12 — Auto-save flip safety.** When `isReadOnly` becomes true, `useAutoSave` must:
    1. Clear all pending timers.
    2. Drop `pending[*]` entries.
    3. Call `currentAbort.abort()` to cancel in-flight PATCH.
    4. Set `relationsDirty = false`.
    5. Refetch the entity from server and call `mergeServerResponse`.
    6. **NOT** transition `status` to `"error"` — this is a graceful state change, not a failure.
    7. Surface a toast: "Server is now read-only — your unsaved changes have been discarded." (Or kept; see AC13.)
    - **Test:** `useAutoSave.test.ts::TestFlipToReadOnly_AbortsAndClears` — pending timer, in-flight PATCH, both cancelled; entity refetched; status === `"idle"`, not `"error"`.

13. **AC13 — Unsaved work preservation.** When `isReadOnly` flips while the user has dirty form state (typed text not yet flushed): formData is preserved client-side, but flagged as `unsavable`. Toast: "Server is now read-only — your unsaved changes haven't been written. Copy them now if you need them." The form's Save button (already hidden by AC10) does not re-appear. A future SSE refresh that returns the server to open mode re-enables saves; the user can click Save then.
    - **Test:** `DynamicForm.test.ts::TestReadOnlyFlip_PreservesDirtyState` — type into a field, flip read-only, formData still contains the typed text, toast appears.

14. **AC14 — Modals close on flip.** Any open modal with write intent (CreateModal, DeleteConfirm, command runner) closes itself on read-only flip and shows a toast naming why. Read-only modals (help, view-info) stay open.
    - **Test:** per-modal unit test.

#### Enforcement & cross-cutting

15. **AC15 — ESLint rule + `data-testid` convention.** New ESLint rule `rela/write-affordance-must-gate` (or use a custom file-pattern rule) that flags `@click` handlers calling `entitiesStore.create|update|delete|...` (or any `frontend/src/api/*.ts` write function) without a `v-if` referencing `useACL()`. Plus a `data-testid="write-affordance"` convention on every gated element so the E2E test can sweep for them.
    - **Test:** ESLint rule itself has unit tests; one repo-wide lint check passes after the migration.

16. **AC16 — E2E coverage.**
    - Extend `e2e/tests/fixtures.ts::spawnServer` to accept `extraArgs: string[]`.
    - Add a `readOnlyServerUrl` fixture (parametrised over the same memstore-backed project).
    - One new Playwright test: boots `rela-server --read-only`, navigates to a list, asserts (a) banner visible, (b) no `[data-testid="write-affordance"]` elements are reachable, (c) typing into a form field doesn't trigger a network call within the debounce window.
    - **Test plan:** the test itself IS the AC verification.

17. **AC17 — Open-mode regression.** When `rela-server` runs without `--read-only`, the entire existing E2E suite passes unchanged. All happy-path create/update/delete flows still work.
    - **Test:** `just e2e` exits 0 against develop's existing fixtures.

18. **AC18 — Backwards-compat with older server.** Frontend treats `_config.acl` absence as `mode: "open"`. No banner, no gating. (Older server pre-this-PR.)
    - **Test:** `aclStore.test.ts::TestServerWithoutACLField_DefaultsToOpen`.

19. **AC19 — Live-transition staleness window documented.** If SSE disconnects and reconnects, the SPA may not learn of a server restart until the next `refresh` event. The plan accepts this staleness window (typically seconds). Documented in `docs/security.md` so operators understand the UX guarantees.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing patterns:**

- **`entitymanager.Deps` pattern** (`internal/entitymanager/manager.go`) — the model for `dataentry.NewApp(Deps)`. Required fields nil-checked in constructor; `audit.Nop{}` / `acl.NopACL{}` are the explicit opt-outs.
- **`appbuild.Services.ACL()` accessor** (already exists) — the wiring source. `cmd/rela-server/main.go` reads it.
- **`mcp.Services` consumer-side interface** (`internal/mcp/server.go`) — the eventual target shape for `dataentry`; out of scope per the architect audit, tracked as a follow-up.
- **Pinia store + composable wrapper** — established pattern (`useSchemaStore`, `useUIStore`); `useACL`+`aclStore` mirror it.
- **`useAutoSave` internals** (`frontend/src/composables/useAutoSave.ts`) — `pending`, `currentAbort`, `queueTail`, `mergeServerResponse`. The flip-safety implementation hooks all three.
- **`useEvents` SSE handlers** — currently invalidates entities + git status; extending to also reload `aclStore` is two lines.

**Survey** (delivered by the in-flight agent, summarized above and below)
enumerated 31 distinct write affordances in 9 component areas plus 5 keyboard
shortcuts.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

### Backend

```go
// internal/dataentry/api_v1.go

type V1ACLConfig struct {
    // Mode reports the active ACL's behavior class:
    //   "open"      — every authenticated request can write (NopACL or
    //                 *Declarative in v0 — see below).
    //   "read-only" — every write returns 403 (ReadOnlyACL).
    //
    // **v0 → v1 progression.** v0 emits "open" or "read-only" only.
    // v1 will introduce "policy" + a populated WritesAllowedFor list
    // when *Declarative gains per-type gating in the SPA. Until then,
    // *Declarative deliberately surfaces as "open" so the SPA's existing
    // 403 + toast path handles per-type denies — same UX as today's
    // pre-ACL deployments. See docs/security.md.
    Mode string `json:"mode"`
}

func (a *App) handleV1Config(w http.ResponseWriter, r *http.Request) {
    // ...existing...
    config := V1Config{
        // ...
        ACL: V1ACLConfig{Mode: aclMode(a.acl)},
    }
    writeV1JSON(w, http.StatusOK, config)
}

// aclMode classifies the active ACL for the SPA wire contract.
// Type-asserts rather than adding a Mode() string method to acl.ACL —
// the strings are dataentry's vocabulary, not the ACL package's.
// Explicit cases per impl so the v0→v1 migration is a single labelled
// edit, not "find the default branch and remember what it hid."
func aclMode(a acl.ACL) string {
    switch a.(type) {
    case acl.NopACL:
        return "open"
    case acl.ReadOnlyACL:
        return "read-only"
    case *acl.Declarative:
        // v0: SPA can't act on per-type policy yet; reports as "open"
        // and relies on the standard 403 + toast (TKT-JZRNF) for
        // per-type denies. v1 returns "policy" once writes_allowed_for
        // is on the wire.
        return "open"
    default:
        slog.Warn("dataentry: unknown acl.ACL implementation; reporting mode=open",
            "type", fmt.Sprintf("%T", a))
        return "open"
    }
}
```

```go
// internal/dataentry/app.go

type Deps struct {
    FS            storage.FS
    Paths         *project.Context
    Meta          *metamodel.Metamodel
    Store         store.Store
    EntityManager entitymanager.EntityManager
    Searcher      search.Searcher
    ACL           acl.ACL
}

func NewApp(d Deps) (*App, error) {
    if d.FS == nil      { return nil, errors.New("dataentry.NewApp: FS is required") }
    if d.Paths == nil   { return nil, errors.New("dataentry.NewApp: Paths is required") }
    if d.Meta == nil    { return nil, errors.New("dataentry.NewApp: Meta is required") }
    if d.Store == nil   { return nil, errors.New("dataentry.NewApp: Store is required") }
    if d.EntityManager == nil { return nil, errors.New("dataentry.NewApp: EntityManager is required") }
    if d.Searcher == nil      { return nil, errors.New("dataentry.NewApp: Searcher is required") }
    if d.ACL == nil           { return nil, errors.New("dataentry.NewApp: ACL is required (use acl.NopACL{} to opt out)") }
    // ...existing setup, using d.* references...
}
```

### Frontend — store + composable

```ts
// frontend/src/stores/acl.ts
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getConfig } from '@/api/schema'

export type ACLMode = 'open' | 'read-only' | 'policy'

export const useACLStore = defineStore('acl', () => {
  const mode = ref<ACLMode>('read-only')  // fail-safe default — see useACL ready behavior
  const loaded = ref(false)
  let loadPromise: Promise<void> | null = null

  async function load() {
    if (loaded.value) return
    if (loadPromise) return loadPromise
    loadPromise = (async () => {
      try {
        const cfg = await getConfig()
        mode.value = (cfg.acl?.mode as ACLMode) ?? 'open'  // absent → open (AC18)
        loaded.value = true
      } catch (e) {
        // Network errors mean we can't verify ACL state. Fail safe to
        // read-only so users don't see write affordances against a
        // potentially-locked server.
        mode.value = 'read-only'
        loaded.value = true  // still mark loaded; we made our best effort
      } finally {
        loadPromise = null
      }
    })()
    return loadPromise
  }

  async function reload() {
    loaded.value = false
    return load()
  }

  return { mode, loaded, load, reload }
})
```

```ts
// frontend/src/composables/useACL.ts
import { computed } from 'vue'
import { useACLStore } from '@/stores/acl'

export function useACL() {
  const store = useACLStore()
  const ready = computed(() => store.loaded)
  const mode = computed(() => store.mode)
  // FOUC prevention: until loaded, behave as read-only so write
  // affordances stay hidden. The schema-store gate in App.vue already
  // delays mounting most views until load completes, so in practice
  // this is invisible. The fail-safe matters for the open-mode flash
  // and for network errors during _config.
  const isReadOnly = computed(() => !ready.value || mode.value === 'read-only')
  const isOpen = computed(() => ready.value && mode.value === 'open')
  return { mode, isReadOnly, isOpen, ready }
}
```

Reasoning for the fail-safe (cranky finding): a brief render of "+ Create"
buttons that then vanish is worse UX than waiting on the gate. The schema store
already gates most view mounting, so the fail-safe is invisible in practice.

### Frontend — banner

`frontend/src/components/common/ReadOnlyBanner.vue` — ~40 lines.
`role="status"`, `aria-live="polite"`, calm color (e.g. neutral border-bottom,
neutral background — not error red, not warning amber), WCAG AA contrast. Copy
lives in a constant `READ_ONLY_BANNER_COPY` exported from the component so
docs/security.md can reference it.

Mounted in `App.vue`:

```vue
<template>
  <div class="app-root">
    <ReadOnlyBanner v-if="useACL().isReadOnly" />
    <!-- existing app shell -->
  </div>
</template>
```

### Frontend — `useAutoSave` flip safety

```ts
// frontend/src/composables/useAutoSave.ts (additions)

export function useAutoSave({ entityId, isReadOnly /* new */ }) {
  // ... existing state ...

  watch(isReadOnly, async (readOnly) => {
    if (!readOnly) return
    // 1. Clear pending timers
    for (const t of timers.values()) clearTimeout(t)
    timers.clear()
    // 2. Drop pending entries
    pending.clear()
    // 3. Abort in-flight PATCH
    currentAbort?.abort()
    currentAbort = null
    // 4. Reset relations dirty flag
    relationsDirty.value = false
    // 5. Refetch entity, repopulate
    if (entityId.value) {
      try {
        const fresh = await entitiesStore.fetchById(entityId.value, { skipCache: true })
        mergeServerResponse(fresh)
      } catch {
        // Server may be unreachable; that's fine, formData stays as-is
      }
    }
    // 6. status -> 'idle' (graceful), not 'error'
    status.value = 'idle'
    // 7. Toast
    uiStore.toast('Server is now read-only. Unsaved changes preserved locally but cannot be saved.')
  })

  function scheduleFieldSave(property: string, value: unknown) {
    if (isReadOnly.value) return  // belt + suspenders against ungated callers
    // ...existing...
  }

  function fireProperty(property: string) {
    if (isReadOnly.value) return  // check inside firing fn, not at scheduling time
    // ...existing PATCH...
  }
}
```

### Frontend — affordance gating defaults

Per affordance category, the gating mechanism:

- **Hidden via `v-if="!useACL().isReadOnly"`** — buttons, menu items, action rows, sidebar items, banner items.
- **Replaced with static rendering** — chip pickers, click-to-edit headers, inline editors. The static view stays useful; only the editor entry is gated.
- **HTML `readonly` attribute** — `<input>`, `<textarea>` plain fields.
- **CodeMirror/EasyMDE readonly mode** — `MarkdownEditor` uses EasyMDE; `editor.toggleReadOnly(true)` (not the HTML attribute).
- **Suppress event handlers** — `draggable="false"` for kanban cards; `@click` removed for inline checkboxes; `pointer-events: none` for content checkboxes; keyboard composables consult `useACL()` and short-circuit.
- **Compose via `useAutoSave({ isReadOnly })`** — auto-save respects the prop, no per-component gating needed inside `useAutoSave` callers.

### Files to modify (v2 — concrete from survey)

**Backend:**

| File | Change |
|---|---|
| `internal/dataentry/api_v1.go` | Add `V1ACLConfig`; embed in `V1Config`; populate in `handleV1Config`; add `aclMode()` helper |
| `internal/dataentry/app.go` | Convert `NewApp(...)` → `NewApp(Deps)`; stash `acl` on App |
| `internal/dataentry/api_v1_config_acl_test.go` | NEW — AC1+AC3 tests, compile-time guard |
| `internal/dataentry/app_test.go` | Update to use `NewApp(Deps{})` — test nil-rejection for each required field |
| `internal/dataentry/*_test.go` | Migrate to new `Deps`-shaped constructor wherever `NewApp` is called |
| `cmd/rela-server/main.go` | Build `Deps{...}` from `svc`; pass to `dataentry.NewApp` |
| `cmd/rela-desktop/main.go` | Same |
| `docs/security.md` | New subsection "Read-only mode is a UX cue, not a security control" + the v0/v1 progression for `mode` |

**Frontend — store/composable/banner:**

| File | Change |
|---|---|
| `frontend/src/stores/acl.ts` | NEW |
| `frontend/src/stores/acl.test.ts` | NEW |
| `frontend/src/composables/useACL.ts` | NEW |
| `frontend/src/composables/useACL.test.ts` | NEW |
| `frontend/src/composables/useEvents.ts` | Hook ACL reload into `refresh` event |
| `frontend/src/composables/useEvents.test.ts` | Assert ACL reload triggered |
| `frontend/src/composables/useAutoSave.ts` | Accept `isReadOnly`; watcher + per-fn guards |
| `frontend/src/composables/useAutoSave.test.ts` | AC12 + AC13 |
| `frontend/src/components/common/ReadOnlyBanner.vue` | NEW |
| `frontend/src/App.vue` | Mount banner |
| `frontend/src/App.test.ts` | AC9 snapshots |
| `frontend/src/api/schema.ts` | Update `Config` type to include `acl?: ACLConfig` |
| `frontend/src/types/config.ts` | Add `ACLConfig` |
| `frontend/CLAUDE.md` | New section "ACL-gated affordances — read this before adding a write button" |

**Frontend — per-affordance (31 sites, 9 components):**

| File | Affordances gated |
|---|---|
| `frontend/src/components/lists/EntityList.vue` | "+ New" button + `n` shortcut, row delete + `Del`/`Backspace` shortcuts, bulk action row + checkboxes + single-letter shortcuts, `e` shortcut |
| `frontend/src/components/entity/EntityDetail.vue` | Desktop+mobile Edit, desktop+mobile Delete, command buttons + overflow menu, content checkboxes (`setupCheckboxHandlers`), per-row Edit pencils, `e` and `Del` shortcuts |
| `frontend/src/components/forms/DynamicForm.vue` | Save/Create button + `Cmd+Enter`, Cancel, autosave (via `useAutoSave({isReadOnly})`), template selector, ID controls, beforeunload guard, route guard |
| `frontend/src/components/forms/RelationCards.vue` | Per-card `×`, per-property inputs (replaced with display), "+ Add" button, "Link" button |
| `frontend/src/components/forms/RelationPicker.vue` | Chip `×`, search input + dropdown, "+ Add new" footer buttons |
| `frontend/src/components/forms/InlineCreateModal.vue` | Defensive guard in `show` watcher; entry points already gated upstream |
| `frontend/src/components/forms/SidePanel.vue` | "+ Add" buttons |
| `frontend/src/components/forms/FieldRenderer.vue` | Propagate `readonly` to each rendered field type; EasyMDE `toggleReadOnly` |
| `frontend/src/views/KanbanView.vue` | "+ New" header, `draggable="true"` removal + drop handlers |
| `frontend/src/views/SettingsView.vue` | User-defaults `<form>` body via `<fieldset disabled>`, Save/Reset row, palette Save, logo upload/remove, theme Install, git commit |
| `frontend/src/views/ConflictsView.vue` | "Apply resolution" |
| `frontend/src/components/common/StatusBar.vue` | Git sync click handler |
| `frontend/src/components/common/Sidebar.vue` | Action items in nav |
| `frontend/src/components/modals/CommandModal.vue` | Defensive `runCommand` guard |
| `frontend/src/composables/useListKeyboard.ts` | Consult `useACL`, short-circuit on read-only |
| `frontend/src/composables/useListActions.ts` | Same |
| `frontend/src/composables/useKeyboardShortcuts.ts` | Same |

**Enforcement + E2E:**

| File | Change |
|---|---|
| `frontend/eslint.config.js` (or equivalent) | NEW rule `rela/write-affordance-must-gate` |
| `frontend/src/styles/banner.css` (or scoped in component) | Banner styles |
| `e2e/tests/fixtures.ts` | Extend `spawnServer` to accept `extraArgs`; add `readOnlyServerUrl` fixture |
| `e2e/tests/read-only.spec.ts` | NEW — AC16 |

### Alternatives considered (v2)

| Alternative | Rejected because |
|---|---|
| Add `Mode() string` method to `acl.ACL` interface | Leaks dataentry's wire vocabulary into the ACL package; violates consumer-side-interface rule |
| Add `ACL()` method to `entitymanager.EntityManager` interface | EntityManager is transitional (CLAUDE.md); grows surface for every consumer |
| Consumer-side `dataentry.AppServices` interface NOW | Deferred — significant refactor; tracked as follow-up. `Deps` struct is the right step in this PR |
| Disable buttons rather than hide | Doesn't communicate "this server is read-only" clearly; hide + banner is the calm UX |
| Banner with "Dismiss" button | State isn't dismissible; reappearing on next route would be confusing |
| Per-route gating | Doesn't compose; composable + per-component check is the right grain |
| Pointer-based `*V1ACLConfig` | Adds nil-checks for a non-use-case; value with omitempty would also work but introducing inconsistency; **plain value, always emitted** is simplest |
| `WritesAllowedFor` in v0 wire shape (with omitempty) | Removed entirely from v0 wire — v1 reviewer might populate accidentally and leak taxonomy. v1 ticket adds the field with full design |
| Store ACL in `schemaStore` | Conflates "what entities exist" with "what is server policy"; different lifecycles; dedicated `aclStore` is the right boundary |
| Default `isReadOnly = false` until loaded | FOUC: write buttons flash then disappear in read-only deployments. Fail-safe to `isReadOnly = true` until ready |
| Per-affordance Vitest snapshots as enforcement | Snapshots rot; new affordances slip through. ESLint rule + `data-testid` is enforceable |
| Static `isReadOnly` destructured at composable construction | Bug bait: stale value at fire time. Always read `isReadOnly.value` inside firing functions |
| Optimistic UI updates with rollback on 403 | Already what `useAutoSave` does; the flip case needs an explicit reset path because the rollback assumes the server has the truth — which it does, but the formData on the client doesn't match anymore |

### Dependencies

- `internal/acl` — concrete types `NopACL`, `ReadOnlyACL`, `*Declarative` (all stable; see compile-time guard in tests).
- `internal/appbuild` — `Services.ACL()` accessor (already exists).
- No new third-party deps (Go or npm).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input sources:**

| Input | Source | Validation |
|---|---|---|
| `acl.ACL` instance | `appbuild.Services` | Trusted; type assertion contract |
| `acl.mode` from server | API response | Frontend treats unknown strings as `"open"` to avoid blocking users; whitelist `open`/`read-only`/`policy` |
| `_config` field absence | Older server | Default `"open"` (AC18) |

**Threat model — banner is decorative.** The actual gate is the backend's 403.
An attacker can edit the JS bundle to bypass `isReadOnly` and attempt writes;
they will receive 403s. This is **documented in `docs/security.md` as a section
titled "Read-only mode is a UX cue, NOT a security control"** so operators don't
mistake the UI banner for a security boundary.

**No new attack surface from `acl.mode`.** The mode is derivable by a single
probe write anyway. Surfacing it in `_config` is honesty.

**v1 risk for `WritesAllowedFor`:** when v1 populates the field, it will leak
both the type taxonomy AND the principal's permissions to anyone who can reach
the SPA. v1 must answer two questions before populating: (a) is the type
taxonomy itself permission-scoped (does an unauthorized user even know type `X`
exists)? (b) is the per-principal list scoped to the requesting principal only
or to the wider population? v0 does NOT add the field to the Go struct — it's
reserved on the wire spec but not implemented — so a v1 reviewer can't
accidentally populate it.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

| AC | Test file/name |
|---|---|
| AC1 | `internal/dataentry/api_v1_config_acl_test.go::TestConfig_ACLMode_*` (4 cases + warning case) |
| AC2 | `internal/dataentry/api_v1_test.go` — additive assertion |
| AC3 | `TestACLMode_*` + compile-time guard |
| AC4 | `internal/dataentry/app_test.go::TestNewApp_Deps_*` |
| AC5 | `frontend/src/stores/acl.test.ts` |
| AC6 | `frontend/src/composables/useACL.test.ts` |
| AC7 | `frontend/src/App.test.ts::TestNoWriteAffordances_BeforeACLReady` |
| AC8 | `frontend/src/composables/useEvents.test.ts::TestRefreshEvent_TriggersACLReload` |
| AC9 | `frontend/src/components/common/ReadOnlyBanner.test.ts` + App.test.ts |
| AC10 | per-component Vitest (17 files) — for each affordance group, "renders nothing under read-only" |
| AC11 | `useListKeyboard.test.ts`, `useListActions.test.ts`, `useKeyboardShortcuts.test.ts`, `DynamicForm.test.ts::handleKeydown` |
| AC12 | `useAutoSave.test.ts::TestFlipToReadOnly_AbortsAndClears` |
| AC13 | `DynamicForm.test.ts::TestReadOnlyFlip_PreservesDirtyState` |
| AC14 | per-modal unit test |
| AC15 | ESLint rule self-tests + lint pass |
| AC16 | `e2e/tests/read-only.spec.ts` |
| AC17 | `just e2e` exits 0 against existing suite |
| AC18 | `aclStore.test.ts::TestServerWithoutACLField_DefaultsToOpen` |
| AC19 | Documentation review — no test, manual audit |

**Edge cases (v2):**

- `acl.mode` is an unknown string (server speaks ahead of client, e.g. `"strict"`) → frontend treats as `open` (least disruptive). Documented.
- Server enters read-only mode after the SPA has loaded → SSE `refresh` event triggers `aclStore.reload()` (AC8).
- `useAutoSave` flip with in-flight PATCH → AC12 covers; abort + refetch + repopulate.
- Multiple browser tabs against the same server → each tab's SSE fires independently; each tab's autosave handles its own dirty state.
- Tab/window not in focus → SSE still fires; banner appears on next focus.
- SSE disconnects and reconnects without firing `refresh` → SPA stays on stale ACL state until next refresh event. Documented as "staleness window" in `docs/security.md` (AC19).
- Network error during `_config` fetch → `aclStore` fails safe to `"read-only"` (AC5); user sees the banner; on next reload (or retry) the real state is fetched.
- Confirm dialog open when read-only flips → modal close + toast (AC14).
- Optimistic UI update applied before read-only flip → refetch in AC12 reconciles.
- ESLint rule false positives → suppress with explicit comment + reviewer approval. Documented in CLAUDE.md.

**Negative tests:**

- Backend `aclMode(nil)` → returns `"open"` + warn log.
- Frontend `useACL()` with no schema/acl store loaded → `isReadOnly === true` until ready.
- Open-mode E2E: every existing write test passes unchanged.
- Read-only E2E: every write affordance is unreachable, every write API call from a kept-open client returns 403.

**Integration tests:**

- One Playwright E2E (AC16) is the integration test for the full chain: server flag → backend `_config` → SPA `aclStore` → banner + hidden affordances.
- AC17 (existing E2E suite) is the regression net.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Effort:** `l` (5-8 days). The plan is now substantial — survey enumerated 31
affordances + 5 keyboard handlers + auto-save flip semantics + ESLint rule + E2E
fixture extension. Backend is < 1 day; bulk is frontend.

**Risks (v2):**

| Risk | Severity | Mitigation |
|---|---|---|
| New write affordance added in a future PR escapes the gate | High | ESLint rule + `data-testid` convention (AC15) + frontend CLAUDE.md note |
| `useAutoSave` flip leaves formData desynced from server | High | AC12 explicit; refetch + repopulate; toast |
| Unsaved work lost on flip | High | AC13: formData preserved; toast surfaces; user can copy |
| Modal still rendering write controls after flip | Medium | AC14: modals close + toast |
| Keyboard shortcut triggers write despite hidden button | High | AC11: keyboard composables consult `useACL` |
| FOUC: brief "+ Create" flash before banner appears | Medium | AC7: fail-safe to `isReadOnly = true` until ready |
| SSE-disconnect staleness window | Low | Documented (AC19); next `refresh` event reconciles |
| `mode: "open"` for Declarative confuses operators | Medium | Documented in security.md + Go doc comment + risk-section table here |
| `Deps` struct migration breaks downstream test fixtures | Medium | Compile error forces every callsite to migrate; CI catches |
| ESLint rule has false positives | Medium | Allowlist comment + reviewer approval; documented |
| MarkdownEditor (EasyMDE) `readonly` attr is ignored | Low | Use `editor.toggleReadOnly(true)` per-widget; tested |
| Banner color doesn't pass WCAG AA | Low | Color picker + contrast checker before merge |
| `Services` consumer-side interface refactor deferred | Low | Tracked as follow-up; doesn't block v1 |
| Live operator-toggles | Low | AC8 covers; documented staleness window in AC19 |

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

| Doc | Update |
|---|---|
| `docs/security.md` | New section "Read-only mode is a UX cue, NOT a security control" + how the SPA gates per mode + v0/v1 progression for `mode` + the staleness window |
| `frontend/CLAUDE.md` | New section "ACL-gated affordances" — every write affordance must consult `useACL()` and have a `data-testid="write-affordance"` attribute |
| Wire-format docs for `_config` (if any) | Add `acl.mode` |

CLI help: N/A.

## Design Review

- [x] Run `/design-review` before starting implementation (covered by parallel agents)
- [x] All critical/significant findings addressed in plan

**Design Review Findings (v2):**

Three agents audited the plan in parallel before implementation. All findings
folded into v2 above.

**go-architect findings:**

- **F1 (minor → taken)**: Explicit `*acl.Declarative` case in `aclMode()` rather than letting it fall to default. **Folded into AC1 and backend code sketch.**
- **F2 (significant → taken)**: Convert `NewApp` to `Deps` struct. **Folded into AC4; concrete code sketch in Approach.**
- **F3 (minor → deferred with ticket)**: Consumer-side `dataentry.AppServices` interface. **Listed as out-of-scope follow-up.**
- **F4 (minor → taken with concrete test)**: Declarative→open default deserves end-to-end smoke test. **Folded into AC1 (Declarative case in unit test) + the existing TKT-JZRNF toast handling.**
- **F5 (minor → taken)**: Value not pointer for `V1ACLConfig`. **Folded into backend code sketch.**
- **F6 (nit → sound as planned)**: Test placement in new file. **Kept.**
- **F7 (significant → taken)**: `aclMode` package-private with explicit comment. **Folded into backend code sketch.**

**cranky-code-reviewer findings:**

- **F1 (significant → taken)**: "Inventory awaiting survey" is hand-waving. **Survey landed during planning; files-to-modify section is now concrete with all 31 affordances enumerated.**
- **F2 (significant → taken)**: AC5 conflates `readonly` HTML attr with auto-save short-circuit. **AC10 distinguishes per-widget mechanisms; EasyMDE explicitly uses `toggleReadOnly`.**
- **F3 (critical → taken)**: Auto-save flip must abort in-flight, not just block scheduling. **AC12 explicit; 7-step procedure in code sketch.**
- **F4 (significant → taken)**: FOUC during initial load. **AC7: `isReadOnly` defaults to `true` until ready.**
- **F5 (significant → taken)**: Declarative→open UX cost glossed over. **Risk table + docs/security.md note; per AC1 unit test verifies the wire value.**
- **F6 (significant → taken)**: Live transitions: SSE doesn't currently reload schema. **AC8 hooks `aclStore.reload()` into the `refresh` event.**
- **F7 (critical → taken)**: Unsaved work on flip. **AC13: formData preserved + toast.**
- **F8 (significant → taken)**: "Per-affordance Vitest snapshots" unenforceable. **AC15: ESLint rule + `data-testid` convention.**
- **F9 (significant → taken)**: E2E fixture doesn't support `--read-only`. **AC16 extends fixture.**
- **F10 (significant → taken)**: ACL state in schemaStore conflates concerns. **AC5: dedicated `aclStore`.**
- **F11 (significant → taken)**: Keyboard shortcuts not in AC list. **AC11 added.**
- **F12 (significant → taken)**: Banner a11y attributes. **AC9 specifies role/aria-live/WCAG.**
- **F13 (minor → taken)**: `WritesAllowedFor` not in v0 wire. **Removed from v0 Go struct.**
- **F14 (significant → taken)**: Modal flip behavior. **AC14.**
- **F15 (minor → taken)**: SSE-reconnect staleness window. **AC19 documents it.**

**Will record any additional `/code-review` review-response IDs here once the
agent runs against the implementation, not against this plan.**
