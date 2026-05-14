---
id: PLAN-JTLN
type: planning-checklist
title: 'Planning: Inline backtick-triggered entity-reference autocomplete in markdown editor'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- An inline autocomplete on the EasyMDE-backed `MarkdownEditor.vue` that
fires on backtick keystrokes in valid contexts, opens after a delay, shows
prefixes → ids two-phase, inserts `` `<id>` `` on pick, and dismisses on any of
the documented exit conditions.
- Reuse of TKT-I5NO's `insertEntityRef  ` helper for the final write to the
buffer (so adjacency padding, denylist validation, and null-editor safety all
carry over).
- Reuse of `searchEntities()  ` for phase-2 queries scoped by entity type
(the API already supports a `type  ` filter argument).
- Toolbar button from TKT-I5NO stays — the inline path is an addition,
not a replacement. Both insert via the same helper.

Out of scope:

- Mobile/virtual-keyboard ergonomics (separate concern).
- Cursor-context re-trigger ("position cursor after existing `  ` `" →
open). Too heuristic; the toolbar button is the deterministic re-entry.
- `[[...]]  ` wiki-style syntax (IDEA-011).
- Per-user preference UI for the open-delay (constant for now; a future
ticket can expose it via `.rela/user-defaults.yaml  `).
- Server-side changes (no new endpoint).

**Acceptance Criteria:**

1. **Trigger in prose.** Typing `  ` ` in normal paragraph/heading/list/
blockquote text opens the popup after the open-delay grace period.
2. **Suppress in code contexts.** Typing `  ` ` inside a fenced code
block, indented code block, link URL, raw HTML block, or as a closing backtick
does NOT open the popup.
3. **Open delay.** Default 600 ms; constant in source. Typing a non-ID
character before the delay elapses cancels the open. No popup flash.
4. **Phase 1 — prefix list.** Popup opens listing every entity type's
`id_prefix  ` (or `id_prefixes  `) from the metamodel, plus a `(manual)  ` row
for each `id_type: manual  ` type. Filtered by what the user has typed since the
trigger backtick.
5. **Phase 2 — id list.** When typed text exactly matches a prefix
(e.g. `TKT-  `), OR the user picks a prefix with Enter, the popup transitions to
listing entity IDs of that type. Backend is `searchEntities(partialQuery,
entityType)  ` — already supported.
6. **Keyboard.** ArrowDown/Up navigates the list with wrap-around; Enter
picks the highlighted row; Esc dismisses without inserting.
7. **Non-focus-stealing.** Document focus stays on the CodeMirror
textarea throughout. Author can continue typing while popup is open (typing
filters the list); only Arrow/Enter/Esc are diverted to popup navigation, all
other keys pass through to the editor.
8. **Insert.** Picking inserts `` `<id>` `` replacing the range
`[triggerPos, cursor]  ` via TKT-I5NO's `insertEntityRef  ` helper. Cursor lands
after the closing backtick. The auto-paired closing backtick that
EasyMDE/CodeMirror inserts (the `   ` `pair) is consumed correctly so we don't
end up with  ` `   `<id>`  `  `.
9. **Auto-dismiss.** Popup closes silently on: typing space, punctuation,
or another `  ` ` (closing); backspacing past the trigger; cursor moving off the
trigger line; editor blur.
10. **Coexistence.** TKT-I5NO toolbar picker still works. Both paths
produce the same insertion. Existing TKT-I5NO unit tests stay green.
11. **Unit tests.** Vitest for: trigger detection (open + 4 suppress
cases); two-phase filter logic; auto-dismiss rules; insertion calls
`insertEntityRef  ` with the correct range.
12. **Playwright e2e.** Happy path (type backtick + prefix + id, save,
verify rendered link) plus negative cases (inside fenced block, close code
span).

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing solutions:**

- **CodeMirror v5 `show-hint  ` addon** ships with EasyMDE. Provides anchored
popup, debounce, list rendering. We **considered** using it but rejected for
this ticket: `show-hint  `'s default flow is to steal focus to the popup (its
own keyboard handlers wrap the editor), and adapting it to the two-phase +
non-focus-stealing model is more work than rolling our own popup component
reusing the existing `EntityPickerModal  ` styles.
- **`EntityPickerModal.vue  `** (TKT-I5NO) — has the result-list rendering
pattern, ARIA listbox attrs, abort-on-close, debounce. We extract the rendering
parts into a reusable `EntityPickerList  ` subcomponent so both the modal and
the inline popup share the visual + a11y bits.
- **`searchEntities(query, type, signal)  `** — `api/entities.ts  `. Already
accepts an optional `type  ` filter. Phase 2 uses it.
- **`insertEntityRef  ` helper** — TKT-I5NO. Handles adjacency padding,
denylist validation, null-editor guard. We call it from the inline path; the
result is symmetric with the modal path.
- **`schemaStore.entityTypes  `** — Map of entity-type definitions. Each
carries `id_prefix?: string  `, `id_prefixes?: string[]  `, `id_type?: 'short' |
'sequential' | 'manual'  `. Phase 1 prefix list is derived from this.
- **CodeMirror tokenizer** — verified end-to-end via the static
prototype + Puppeteer that the two-side `getTokenAt  ` rule correctly
discriminates open vs. suppress for fenced blocks, link URLs, and
closing-backtick cases.

**Reference implementations / prior art:**

- GitHub `@mention  ` autocomplete in PR/issue editors — single-trigger,
non-focus-stealing, dismiss on non-mention character.
- VS Code IntelliSense — broader cursor-context trigger; out of scope.
- Notion `/  ` commands — same non-focus-stealing pattern.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical approach:**

### 1. New component: `EntityPickerList.vue  `

Extract the result-list rendering from `EntityPickerModal.vue  `. Pure
presentational: props `items  `, `highlightedIndex  `, `hint?  `; emits `pick  `
and `hover(idx)  `. Reused by:
- `EntityPickerModal  ` (refactored to use the new component internally)
- `BacktickAutocompletePopup  ` (the new inline popup, see §3)

This makes the visual + ARIA contract one source of truth — also addresses
RR-Q7UH from TKT-I5NO's review by starting the "shared search palette core"
refactor that was filed as a follow-up.

### 2. New composable: `useBacktickAutocomplete  `

`frontend/src/composables/useBacktickAutocomplete.ts  `. Takes a CodeMirror
instance and returns a session controller. The composable encapsulates:

- **Trigger detection.** Subscribes to `cm.on('inputRead', …)  `. On a
single-character `  ` ` insertion, classifies via the two-side token rule:
  - `tokAfter = cm.getTokenAt({line, ch: triggerCh + 1})  `
  - `tokBefore = cm.getTokenAt({line, ch: triggerCh - 1})  `
  - Open iff `tokAfter.type  ` contains `"formatting"  ` AND `tokBefore  `
is inline-text context (null/empty, or contains `header  `, `quote  `, `em  `,
`strong  `).
- **Open delay.** `setTimeout  ` of `OPEN_DELAY_MS  ` (default 600 ms).
Cancelled if the user types a non-ID char or `Esc  ` during the delay.
- **State machine.** `idle → pending → prefix → id → idle  `. Each
transition exposed as a reactive ref so the popup component can render off it.
- **Auto-dismiss rules.** `cm.on('change', …)  ` + `cm.on('cursorActivity', …)  `
  + `cm.on('blur', …)  ` close the session on the documented conditions.
- **Key handling.** `cm.on('keydown', …)  ` intercepts Arrow/Enter/Esc
when a session is open; all other keys pass through to CodeMirror.
- **Lifecycle.** Composable returns `dispose()  `; `MarkdownEditor.vue  `
calls it in `onBeforeUnmount  ` before nulling `editor  `.

### 3. New component: `BacktickAutocompletePopup.vue  `

Positioned absolutely, anchored at the trigger backtick's character coords
(`cm.charCoords  `). Renders an `<EntityPickerList>  ` plus a hint header that
reflects the current phase. The popup does NOT capture focus — its mousedown
handler calls `event.preventDefault()  ` to keep focus on CodeMirror, just like
`EntityPickerModal.vue  ` does.

Animation on mount: 100 ms fade-in (same as the modal).

z-index: same value as `EntityPickerModal  `'s overlay (`10000  `) so it also
sits above EasyMDE fullscreen.

### 4. MarkdownEditor wiring

`MarkdownEditor.vue  `:
- Import + register the new composable in `onMounted  ` after creating
the EasyMDE instance: `autocomplete = useBacktickAutocomplete(editor)  `.
- Mount `<BacktickAutocompletePopup>  ` in the template, bound to the
composable's state.
- `onBeforeUnmount  `: call `autocomplete.dispose()  ` before
`editor.toTextArea()  `.

### 5. Prefix → type mapping

Built once at session-open from `schemaStore.entityTypes  `. For each entity
type:
- If `id_prefix  ` set → one prefix entry (e.g. `TKT-  `).
- If `id_prefixes  ` set → one entry per prefix.
- If `id_type === 'manual'  ` → one entry with the type's label as
"prefix" (no actual prefix; selection goes straight to phase 2 with the type
filter, no text inserted in the buffer yet beyond the trigger backtick).

### 6. Phase transitions

- **Pending → prefix:** when the open-delay timer fires AND the typed
text since the trigger is still valid (no non-ID chars).
- **Prefix → id:** when (a) the typed text exactly matches a prefix, or
(b) the user presses Enter on a prefix row. In both cases the prefix is inserted
into the buffer at `[triggerPos+1, cursor]  ` and the phase flips. Phase 2 then
debounces `searchEntities(query, type)  ` on every further keystroke.
- **Id → done:** Enter or click on an id row calls
`insertEntityRef(editor, id)  ` which handles the full `` `<id>` `` insertion
(replacing the trigger range), then the session closes.

### 7. Auto-paired backtick from CodeMirror

When the user types `` ` `` in a normal context, EasyMDE/CodeMirror auto-inserts
a paired closing backtick — so the buffer goes from `see   ` to `see  ` `` and
the cursor sits between them at `ch: triggerPos + 1  `. Our state machine
accommodates this:
- The trigger backtick is at `triggerPos  `.
- The closing auto-pair is at `triggerPos + 1  `.
- The cursor is between them.
- "Typed after trigger" is the range `[triggerPos+1, cursor]  ` — which
is empty initially.
- On final `pick  `, `insertEntityRef  ` replaces `[triggerPos, cursor+1]  `
(i.e. eats both the opening and the auto-paired closing backtick) with `` `<id>`
``.

### 8. Files

New:
- `frontend/src/components/forms/EntityPickerList.vue  ` — extracted list
  + a11y subcomponent.
- `frontend/src/components/forms/BacktickAutocompletePopup.vue  ` — the
inline popup.
- `frontend/src/composables/useBacktickAutocomplete.ts  ` — state machine
  + CodeMirror event wiring.
- `frontend/src/composables/useBacktickAutocomplete.test.ts  ` — unit
tests for trigger detection (mock CM) and state machine.
- `frontend/src/components/forms/BacktickAutocompletePopup.test.ts  ` —
rendering and pick-emit tests.
- `e2e/tests/markdown-editor-backtick-autocomplete.spec.ts  ` — Playwright
matrix.
- `frontend/src/components/forms/EntityPickerList.test.ts  ` — tests for
the extracted component.

Modified:
- `frontend/src/components/forms/MarkdownEditor.vue  ` — compose the
autocomplete, mount the popup.
- `frontend/src/components/forms/EntityPickerModal.vue  ` — refactor to
use `<EntityPickerList>  `. Existing tests stay green (the refactor is
internal).
- `frontend/src/api/entities.ts  ` — confirm `searchEntities  ` already
takes a `type  ` arg (it does; no change needed beyond consumption).

**Alternatives considered (rejected):**

- **CodeMirror `show-hint  ` addon.** Battle-tested, but its focus-stealing
default and the cost of adapting its hint shape to our two-phase model is higher
than rolling our own popup with the existing styles. Also adds the cost of
pulling in show-hint.css to the SPA bundle.
- **Reuse `EntityPickerModal.vue  ` as-is** (just position it differently).
Rejected because it's a modal with a backdrop overlay; converting it to an
anchored popup wholesale is more disruptive than extracting the shared list
subcomponent.
- **Single-phase with all entities flat-listed.** Simpler, but the
prefix-first UX matches what users want when typing `` `TK… `` — they
intuitively expect tickets first, not a mixed list of every entity in the
project.

**Dependencies:** None new. Reuses EasyMDE, CodeMirror v5, marked, the
schemaStore, the entities API. The `EntityPickerList  ` refactor is
self-contained.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input sources & validation:**

- **Search query** (phase 2): user-typed; passed to `/_search?q=…&type=…  `.
Same endpoint as the modal picker and command palette; no new surface.
- **Inserted ID**: routed through `insertEntityRef  `, which carries the
denylist validation from TKT-I5NO (`internal/store/storeutil.ValidateID  `-
matching rule plus backtick/whitespace rejection, 1024-byte cap). No new
validation path.
- **Prefix list**: derived from `schemaStore.entityTypes  `. Same source
the entire SPA already uses. No new surface.
- **No file I/O, no auth, no credentials.** Pure editor-side feature.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test scenarios:**

| AC | Test | Layer |
|----|------|-------|
| 1 (open in prose) | Unit: mock CM with `tokAfter  ` containing `"formatting"  ` and `tokBefore  ` = null → composable transitions idle → pending → prefix after delay | TS |
| 2a (fenced suppress) | Unit: mock CM with `tokAfter  ` = `"comment"  ` (no `formatting  `) → no transition | TS |
| 2b (link URL suppress) | Unit: mock CM with `tokAfter  ` = `"url"  ` → no transition | TS |
| 2c (closing-backtick suppress) | Unit: `tokBefore  ` = `"comment"  ` → no transition | TS |
| 3 (open delay) | Unit: schedule open, advance fake timer past delay → opens. Advance past delay after typing a space → never opens. | TS |
| 4 (phase 1 prefix list) | Unit: prefix list derived from a stub `schemaStore  ` covers `id_prefix  `, `id_prefixes  `, `id_type: manual  ` cases | TS |
| 5 (phase 2 id list) | Unit: typed text exactly matches a prefix → phase transition; verify `searchEntities  ` called with the correct `type  ` arg | TS |
| 6 (keyboard nav) | Unit: ArrowDown wraps; ArrowUp wraps backward; Enter emits pick; Esc dismisses | TS |
| 7 (non-focus-stealing) | Unit: keydown for arbitrary character (e.g. 'a') is NOT consumed — falls through to CM. Verified via spy on `e.preventDefault  `. | TS |
| 8 (insert) | Unit: pick action calls `insertEntityRef(editor, id)  ` with the correct id | TS |
| 9a (auto-dismiss on space) | Unit: composable transitions to idle after typing space | TS |
| 9b (auto-dismiss on cursor move) | Unit: `cursorActivity  ` outside the trigger line closes the session | TS |
| 10 (coexistence) | Existing TKT-I5NO suite still passes after the `EntityPickerList  ` extraction | TS |
| 11 (unit count) | All above ACs covered by Vitest cases | TS |
| 12 (e2e) | Playwright: type backtick + `T  ` + `K  ` + `T  ` + `-  ` → list narrows then transitions; arrow + Enter inserts `` `TKT-XXX` ``; save; verify rendered link. Plus negative: type inside ``` fence → no popup; type `` `foo` `` quickly → no popup. | e2e |

**Edge cases:**

- **EasyMDE auto-pair backtick.** Typing `` ` `` produces `   ` `` with
cursor between. The state machine treats the trigger as `triggerPos  ` and reads
typed text from `triggerPos+1  ` to the current cursor. On pick, the helper
replaces `[triggerPos, cursor+1]  ` (consuming the auto-pair).
- **User undoes the trigger backtick.** `cm.on('change', …)  ` sees the
removal; the session closes via the "backspaced past trigger" branch.
- **Long ID list in phase 2.** Same `MAX_RESULTS=50  ` cap as the modal
picker. Inherited from `searchEntities  `.
- **Empty prefix list (no entity types declared).** Edge case but
possible in a freshly-bootstrapped project. Treat the popup as immediately
closed; the session never reaches phase 1.
- **Concurrent searches in phase 2.** AbortController on each query;
late responses ignored. Same pattern as `EntityPickerModal  `.
- **Editor in fullscreen mode.** Popup at z-index 10000, above the
9999 fullscreen layer. Inherited from `EntityPickerModal  `'s CSS.

**Negative tests:**

- `useBacktickAutocomplete  ` does NOT transition on a non-backtick input
(e.g. typing 'a').
- Typing `  ` ` while a session is already open (rapid double-backtick)
closes the session per the "non-id char" rule — second backtick is the literal
close.
- Resolver/store error during phase 2 search → error message hint in
popup; no insertion; user can dismiss.

**Integration approach:**

- TS unit tests with Vitest + JSDOM. Mock CodeMirror via a thin shim
exposing `getTokenAt  `, `getCursor  `, `setCursor  `, `replaceRange  `,
`getValue `, `on  `/`off  `, `charCoords  `, plus a fake event emitter for
`inputRead  ` / `change  ` / `cursorActivity  ` / `keydown  ` / `blur  `.
- Playwright e2e against the built `rela-server  ` binary. Seed two
entities; type the trigger; assert popup state at each phase via the page-object
helpers (mirrors what TKT-I5NO's e2e already does for the modal picker).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **CodeMirror v5 quirks across EasyMDE versions.** `inputRead  `,
`getTokenAt  `, `charCoords  ` are stable in CM5 and EasyMDE has pinned CM5; low
risk. Pin the prototype-verified rule in code comments so a future EasyMDE
upgrade is a triage signal.
- **The `EntityPickerList  ` extraction touches a shipping component.**
Mitigated by keeping TKT-I5NO's existing test suite green — any change to the
modal's behavior fails CI. The extraction is a pure refactor (no behavior
change), so the tests pin it down.
- **Typing latency.** Keydown → re-tokenize → token lookup → state
update → popup re-render must complete within one frame for fast typing to feel
right. All operations are O(1) per keystroke on modern hardware; no API calls in
phase 1; phase 2 is debounced.
- **Auto-pair backtick edge case.** Documented above; covered by a
dedicated unit test.

**Effort:** `m  `. The CM event wiring is straightforward; most of the work is
in the state machine and the comprehensive test matrix.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation impact:**

- [ ] User guide: brief mention in the data-entry guide that typing
`  ` `in prose opens an entity-reference autocomplete. Discoverability is also
through the toolbar button's tooltip. Create a `docs-checklist ` on the ticket
when transitioning to review unless the user opts out.
- [x] N/A — CLAUDE.md, README.md unaffected.
- [x] N/A — no new CLI flags or API surface.

## Design Review

- [ ] Run `/design-review  ` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- populated after running /design-review -->
