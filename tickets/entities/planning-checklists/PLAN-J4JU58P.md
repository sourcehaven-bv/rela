---
id: PLAN-J4JU58P
type: planning-checklist
title: 'Planning: Mac conventions in rela-desktop: reopen after closing the last window, Find, Print, Show in Finder, Help'
started: "2026-11-13"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: rela-desktop on macOS keeps working after its last window closes (Dock
icon, Open Recent, Finder open, second launch, Clone from Git, a notification
click), the saved maximized state is restored, and the menu bar gains Edit >
Find…, File > Print…, File > Show in Finder and Help > Rela Help. Print styles
in `RlAppShell` and `RlPageHeader` let a page print in full.

Out of scope: a native find bar in the webview, print preview or page setup,
Windows and Linux menu changes beyond Find, Print and Rela Help, and a local
copy of the documentation.

**Acceptance Criteria:**

1. Closing every window and clicking the Dock icon opens a window. Test: ⌘W
the main window, click the Dock icon, a main window opens on the active project.
2. Open Recent and Finder opens work with no window open. Test: ⌘W, then
File > Open Recent > a project, and a double click on a `.rela` file; each opens
a window on that project.
3. Find, Print, Show in Finder and Rela Help are in the menu bar. Test: the
menus list them; ⌘F focuses a list's search box, else opens /search.
4. Printing a list shows the full list without the sidebar. Test: ⌘P on a
long list; the print preview has no sidebar or header buttons and runs past the
first screen.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: effort s, the platform conventions are known)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: the Mac behavior is the platform default that every document app follows)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- Wails v3 already provides what is needed: `Window.GetByName`,
`events.Mac.ApplicationShouldHandleReopen`, `Window.Print`, `Browser.OpenURL`
and the menu roles. No new dependency.
- The menu already sends shell commands to the SPA (`sendShellCommand`,
`useShellCommands.ts`); Find is one more command.
- Window geometry already persists through `desktop.WindowState`.
- Concept `desktop-app` and feature FEAT-KZMB1J.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Drop the `Desktop.win` field. The main window gets the name `main` and is looked
up with `liveMainWindow`; Wails removes a closed window from its manager, so the
lookup fails once the user closed it. `showMainWindow` opens it again under a
mutex, so a Dock click and a Finder open arriving together cannot open two.
Geometry is saved on `WindowClosing` for each new main window. The Dock reopen
event opens a window only when none is left.

Menus: Edit is built from roles plus Find (the EditMenu role has no Find); Print
calls `Window.Print` on the current window; Show in Finder runs `/usr/bin/open
-R -- <path>` for the focused project; Rela Help opens the docs URL.

Alternatives rejected: hiding the window on close instead of closing it (not
what ⌘W does on a Mac, and the window keeps its memory); keeping the field and
resetting it on close (a race between the close event and the next use).

**Files to modify:**

- `cmd/rela-desktop/main.go`, `mainwindow.go` (new), `menu.go`,
`notify.go`, `reveal.go` (new), `mainwindow_test.go` (new)
- `frontend/src/composables/useShellCommands.ts` and its test
- `frontend/packages/rela-components/src/components/layout/RlAppShell.vue`,
`RlPageHeader.vue`
- `docs-project/entities/guides/GUIDE-desktop.md` (and generated
`docs/desktop.md`)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Show in Finder: the path is the focused project's root from the in-process
registry, which the user opened themselves. It must exist (`os.Stat`) and is
passed as one argv element after `--`, so a leading `-` is not a flag. No shell.
- Find: a fixed shell command with no argument from outside.
- Rela Help: a constant URL.

**Security-Sensitive Operations:**

- Running `/usr/bin/open`: absolute path, argv only, 10 s timeout. Errors go
to a local dialog on the user's own machine.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: the window, Dock and print paths need the Wails runtime and a Mac session; they are checked by hand per the acceptance criteria)

**Test Scenarios:**

- Unit: `TestMainWindowOptions` (saved, degenerate, maximized geometry),
`TestShouldOpenOnReopen`, `TestRevealArgs`, `TestFocusedPath`, and the `find`
cases in `useShellCommands.test.ts`.
- Manual: the four acceptance criteria above in a built rela-desktop.

**Edge Cases:**

- A hidden but not closed window on a Dock click: Wails shows it; no second
window.
- A Dock click and a Finder open at the same time: one window (mutex).
- No project open: Show in Finder is disabled.
- Focused window belongs to a project that was closed: falls back to the
active project.
- A page without a search box: Find opens /search.

**Negative Tests:**

- `revealInFinder("")` and a missing path return an error
(`TestRevealInFinder_RefusesNoPath`).
- A degenerate saved geometry is ignored and the window opens centered.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Wails v3 is a nightly; the name lookup relies on Wails removing closed
windows from its manager. Mitigation: manual check of acceptance criterion 1 on
the pinned version.
- The print CSS uses `overflow: visible !important` inside the main area,
which could clip less on paper than on screen. Mitigation: manual print check of
a long list.

Effort: s.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] The desktop guide (`GUIDE-desktop`, generated `docs/desktop.md`): the
new menu items and the reopen behavior.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the implementation came before this plan was written; the design is reviewed with `/code-review` in the review phase)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review was run; see above)

**Design Review Findings:** None.
