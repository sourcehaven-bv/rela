---
id: PLAN-WED3KNV
type: planning-checklist
title: 'Planning: Quick entry form with a global hotkey'
started: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
In: a top-level `quick_entry:` block in data-entry.yaml with `form:` (required,
a create form), `list:` (optional, a list with an `assign:` block whose values
prefill the form) and `shortcut:` (optional desktop accelerator, default
`CmdOrCtrl+Shift+Space`). A `/quick` SPA route shows that form. The web app
opens it with the `q` key (outside inputs). The desktop app registers a global
hotkey for the active project that opens `/quick?popup=1` in a small window,
or focuses the one already open; a save there closes the window.
Out: several quick entry forms, per-user shortcuts, a shortcut editor in the
UI, a tray icon.

**Acceptance Criteria:**
1. Config validation refuses an unknown `form:`, an unknown `list:`, a list without `assign:`, a list over
   another entity type than the form, and an empty or malformed `shortcut:`.
2. `/quick` renders the configured form with the list's `assign:` values
   filled in; with no `quick_entry:` it shows a short "not configured" note.
3. Saving creates the entity through the normal create path; the form then
   resets for the next item (web) or the popup window closes (desktop).
4. `q` on any page outside an input opens `/quick` when configured, and does
   nothing when not configured. Escape on `/quick` goes back.
5. The desktop app registers the shortcut for the active project, and
   re-syncs it when the active project changes or closes. A switch closes an
   open popup.
6. `quick_entry.**` is classified in the configedit allowlist.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: every part reuses an existing mechanism)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: the platform API covers the hard part)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- Wails v3 `App.GlobalShortcut.Register(accelerator, cb)` handles the global
  hotkey on macOS, Windows and Linux (X11 and portal); no new dependency.
- `DynamicForm` embedded mode with `embeddedPrefill` and `embeddedAddAnother`
  is the inline create form the list side panel already uses.
- `getAssign({list})` (TKT-FGE5G3I) answers a list's `assign:` block as a
  create prefill; EntityList turns it into `embeddedPrefill`.
- `Desktop.OpenWindow` and `safeRoute` open validated SPA routes in windows.
- `useKeyboardShortcuts` holds the single-key global shortcuts.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- Go config: `QuickEntry *QuickEntry` on `dataentryconfig.Config` (`yaml/json:
  quick_entry`), with `Form`, `List`, `Shortcut`. Raise the struct's
  plimsoll field cap by one (format mirror). `validate_quickentry.go` checks
  criterion 1, called from `Validate`. The shortcut is checked against a
  subset of the names Wails parses: modifiers CmdOrCtrl, Cmd, Command, Ctrl,
  Alt, Option, Shift, Super joined by `+` with one key (A-Z, 0-9, F1-F12,
  Space), case-insensitive, at least one modifier.
- configedit: `quick_entry.**` editable (names a form, a list and a key; runs
  no code and grants nothing).
- SPA: `QuickEntry` type in types/config.ts; route `/quick` (name `quick`)
  to `views/QuickEntryView.vue`, which reads the config, loads the assign
  prefill, and renders `DynamicForm` embedded. On `inline-created`: in a
  popup, close the window through the Wails runtime; otherwise toast and
  remount for the next item. `inline-cancelled`: close or go back.
  `useKeyboardShortcuts`: `q` pushes `/quick` when configured; Escape on
  `quick` goes back like a form page.
- Desktop: `quickentry.go` with `(d *Desktop) syncQuickEntry()` called
  whenever the active project changes: unregister the old accelerator,
  close an open popup, register the new one. The callback snapshots the
  active app under the read lock and opens `/quick?popup=1` (no /p/ base, so
  the active project serves it) in a 520x640 window through InvokeAsync, or
  focuses the one already open; a mutex guards the popup pointer.
- Alternative rejected: a modal over the current page on the web. A route
  serves both the web and the desktop popup with one component, and the
  shortcut keeps its place in history.

**Files to modify:**
- internal/dataentryconfig/config.go, validate.go, validate_quickentry.go (+ test)
- internal/configedit/allowlist.go
- frontend/src/types/config.ts, router/index.ts, views/QuickEntryView.vue (+ test),
  composables/useKeyboardShortcuts.ts (+ test), i18n en/nl, the shortcuts help modal
- cmd/rela-desktop/quickentry.go (+ test), main.go (call sites)
- docs-project GUIDE-data-entry (quick entry section), desktop guide if any

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `quick_entry:` comes from operator config and is validated at load: form
  and list must exist, the shortcut must match the accelerator grammar
  (allowlist).
- The popup route is a constant built in Go and still passes `safeRoute`.
- The form posts through the existing create endpoint, which authorizes,
  validates and audits as today. The prefill comes from `GET /_assign`, which
  is already gated per principal.

**Security-Sensitive Operations:**
- A global hotkey listens system-wide, but only for the one accelerator; the
  OS delivers no other keystrokes to the app. It opens a window of the local,
  already-authenticated desktop app and writes nothing by itself.
- Threat model: `.ignored/threat-TKT-24L1H36.md`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- Go table test for validation (criterion 1) and the accelerator grammar.
- configedit `TestAllowlist_ClassifiesEveryKey` covers criterion 6.
- QuickEntryView test with a mocked schema store and `getAssign`: prefill
  passed, not-configured note, reset after save, close in popup (2, 3).
- useKeyboardShortcuts test for `q` and Escape (4).
- Desktop: a unit test for the register/unregister decision over a fake
  shortcut registry (5); the live hotkey is checked by hand.

**Edge Cases:**
No `list:`; a list whose `assign:` names an unreadable entity (prefill
fails, the form opens empty); two presses of the hotkey (one window); the
config reloads with another shortcut (re-registered on the next sync); the
accelerator is taken by another app (registration error logged, app runs
on).

**Negative Tests:**
Unknown form or list, list without assign, type mismatch, edit form,
`shortcut: "Q"` (no modifier), `shortcut: "Cmd+Banana"`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- The default accelerator may clash with an OS or app binding (Spotlight
  uses Cmd+Space). Mitigation: the default adds Shift; a clash is logged and
  the operator sets `shortcut:`.
- The live config can change while the app runs. Mitigation: the callback
  reads the current config on each press; the accelerator is re-synced when a
  project loads. A shortcut change takes effect on the next load (documented).
- The global hotkey cannot be tested in CI. Mitigation: keep the callback thin
  and test the decision logic.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - UI changes (GUIDE-data-entry: "Quick entry")

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see the review-responses linked to TKT-24L1H36.

**Threat model addendum (Configure path).** `quick_entry.**` is editable through Configure by `config:edit`
holders. A draft runs `projectsetup.CheckDataEntry`, which calls `validateQuickEntry`, before it is saved,
so a bad or OS-wide shortcut (for example `Cmd+V`) is refused before the desktop app registers it.
Pinned by `TestCheckDataEntry_QuickEntryShortcut`.
