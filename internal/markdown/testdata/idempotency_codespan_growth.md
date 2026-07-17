
## Problem

The modal entity picker shipped in TKT-I5NO is a deliberate-insertion path — the
author has to leave their flow, find the toolbar button (or use a shortcut),
pick, and resume typing. For authors who reference entities frequently in prose,
the modal context-switch interrupts writing rhythm.

GitHub `@mentions`, Slack mentions, Notion's `/` commands, and IDE autocomplete
all establish the convention: a single trigger key opens an inline,
non-focus-stealing popup that the author can ignore (just keep typing) or
interact with (arrows + Enter to pick).

For rela the natural trigger is ` ` ` — the same character that opens an inline
code span — because the TKT-747O resolver already understands backticked IDs.

## Proposal

Wire CodeMirror's `inputRead ` event to detect when the author types a ` ` ` in
a structural context that permits opening a fresh inline code span, then display
a **non-focus-stealing** dropdown anchored to the cursor. The dropdown opens in
a **two-phase** UX:

- **Phase 1 (prefix selection):** list project-defined ID prefixes (`TKT- `,
`FEAT- `, `DEC- `, …) plus manual-ID types. The author can keep typing — as they
type, the list filters to prefixes still matching. Or they can arrow-navigate
and press Enter to pick a prefix.
- **Phase 2 (id selection):** once the typed text matches a known prefix
(exact match, e.g. `TKT- `), or the author picks a prefix from phase 1, the
popup transitions to listing entity IDs of that type with their titles. Same
arrows + Enter to pick.

Picking an entity inserts `` `<id>` `` (replacing the partial typing) and closes
the popup. Esc dismisses without inserting. Typing a non-ID character (space,
punctuation, closing backtick) silently dismisses.

## Open-delay grace period

A configurable delay (default ~600 ms; prototype found 180 ms felt too eager)
sits between the `` ` `` keystroke and the popup actually opening. During the
delay window, if the author types a non-ID character (e.g. the content of a
literal code span like `` `flag-name` ``), the popup never appears. Tunable via
a single constant; consider a per-user setting later.

## Structural-context detection

The popup must **only** open when the typed `` ` `` is starting a fresh inline
code span. It must NOT open when:

- The cursor is inside a fenced code block.
- The cursor is inside an indented code block.
- The cursor is inside a link URL `[text](url)      `.
- The cursor is inside raw HTML.
- The typed backtick is closing an already-open code span (e.g.
`` `foo` ``).

**Approach (validated against EasyMDE in a static prototype, see Notes):** use
CodeMirror's tokenizer rather than regex heuristics. Two tokens discriminate
cleanly:

- `tokAfter = cm.getTokenAt({line, ch: triggerCh + 1})      ` — contains the
type string `"formatting"      ` only when CodeMirror's markdown mode has
classified this backtick as a markdown inline-code-span boundary. Inside a
fenced block, the same backtick is just `"comment"      ` (overlay content) with
no `"formatting"    `.
- `tokBefore = cm.getTokenAt({line, ch: triggerCh - 1})      ` — for the
"closing an existing code span" case: at the end of `` `foo `` the left
neighbour reports `"comment"      ` (the code-span content), so we suppress.

**Rule:** open iff `tokAfter.type      ` contains `"formatting"      ` AND
`tokBefore   ` is inline-text context (i.e. type is null/empty or matches a
prose token like `header      `, `quote      `, `em      `, `strong      `).

This was verified end-to-end via Puppeteer against the static EasyMDE prototype:
fenced-block content lines (any column), inside link URLs, and closing backticks
all suppress correctly; prose openings fire correctly.

## Acceptance criteria

1. **Trigger detection.** Typing `      ` ` in prose context opens the popup
after the open-delay grace period. Typing inside a fenced/indented code block,
link URL, or as a closing backtick does **not** open it.
2. **Open-delay grace.** Default 600 ms (configurable). Typing a non-ID
character before the delay elapses cancels the open; popup never flashes.
3. **Two-phase popup.** Phase 1 lists prefixes; once typed text matches a
prefix, or the author selects a prefix with Enter, phase 2 lists entity IDs of
that type with titles.
4. **Non-focus-stealing.** Author can continue typing while popup is open.
Arrow keys + Enter still work when popup is open (CodeMirror's existing key
handling is overridden inside the popup session). Esc dismisses. The CodeMirror
cursor never leaves the editor.
5. **Insert.** Picking inserts `` `<id>` `` replacing the partial-typed
range (from trigger backtick through the current cursor). Cursor lands after the
closing backtick. Reuses TKT-I5NO's `insertEntityRef      ` helper for
validation
+ adjacency padding.
6. **Auto-dismiss.** Popup closes silently on: typing space/punctuation/
closing backtick, cursor moving off the trigger line, backspacing past the
trigger, editor losing focus.
7. **Per-line state machine.** Multiple distinct trigger sessions on one
line don't bleed into each other; the popup cleanly tears down between sessions.
8. **Compatibility with toolbar picker (TKT-I5NO).** Both entry paths
produce the same `` `<id>` `` insertion via the same helper. Toolbar button
remains the discoverable affordance for users who don't know about the trigger;
the inline autocomplete is the power-user path.
9. **Unit tests.** Vitest cases for: trigger detection (open/suppress
for each structural context using a mocked CodeMirror), two-phase filter logic,
auto-dismiss rules, insertion via the shared helper.
10. **Playwright e2e.** Open the form, type a backtick + prefix + id,
verify the backticked ID lands in the buffer, save, verify the rendered link on
the detail page. Plus negative cases: type inside a fenced block, type a literal
code span — popup must not appear.

## Out of scope

- A new server-side endpoint — reuses `/_search      ` and the same
`searchEntities      ` helper as the modal picker.
- Mobile/virtual-keyboard ergonomics (separate concern; a follow-up
may add a long-press affordance on touch devices).
- Cursor-context re-trigger (e.g. positioning cursor right after an
existing `      ` `). Cranky-reviewer-style heuristics for "user positioned
cursor inside an in-progress reference" are too unpredictable; the toolbar
button is the deterministic re-entry path.
- `[[...]]      ` wiki-style syntax (IDEA-011).

## Notes

A static HTML prototype validated the approach end-to-end (EasyMDE in a browser,
CodeMirror's tokenizer as the structural classifier, Puppeteer- driven test
matrix). The prototype confirmed:

- The "two-side token classification" rule above correctly discriminates
open vs. suppress across all probed contexts.
- 600 ms felt right as the open-delay default — fast typists never see
the popup for literal code spans; deliberate references trigger reliably.
- The two-phase prefix → id transition is intuitive; the prefix list is
short (one per entity type in the project, 4–10 typically) so phase 1 is rarely
a bottleneck.

Depends on TKT-I5NO landing (it ships the `insertEntityRef      ` helper and
`EntityPickerModal      `-derived search wiring this ticket reuses).
