---
id: PLAN-RY25IP
type: planning-checklist
title: 'Planning: Insert and edit external links in the Milkdown editor (plus horizontal rule and undo/redo)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: link insert over a selection and over an empty cursor; retarget an existing
link; unlink keeping the text; paste-a-URL-over-a-selection wraps it; a floating
tooltip on the link showing the URL with edit and unlink actions; a
horizontal-rule toolbar command; undo and redo toolbar buttons.

OUT: image insert (no upload path exists in the editor, so it would be URL-only
and is a separate decision); headings 4-6; the `/` block menu that
`filterBlockCommands` was written for but never wired up; entity targets in the
link dialog (the `@`-mention and entity-ref button already cover internal links
and use a different markdown construct with ACL-gated titles); link `title`
attributes; reference-style links; autolink literals.

**REMOVED FROM SCOPE DURING PLANNING — the keyboard shortcut.** The original AC
7 asked for `Mod-k`. That shortcut is already taken:
`composables/useKeyboardShortcuts.ts:46` binds Cmd/Ctrl+K to the command
palette, and its comment states the bypass of `isInputFocused` is deliberate
("users expect the palette to open from anywhere, including form fields"). The
listener is registered bubble-phase on `document` (`:133`) and tests only
`e.key`, never `defaultPrevented` — so a ProseMirror keymap binding `Mod-k`
would NOT suppress it: the link dialog would open AND the palette would open on
top of it. Resolving this means either taking `Mod-Shift-K` (unfamiliar) or
narrowing a shared composable and its tests (risk disproportionate to this
ticket). Decision: ship the toolbar button and tooltip now, decide the shortcut
separately once the UI exists. Original AC 7 is struck; ACs renumbered below.

**Acceptance Criteria:**

1. With text selected, the link button inserts a link; the body serializes to
`[text](https://example.com)`.
2. With the cursor inside a link, a tooltip appears showing the URL, with edit
and unlink actions.
3. Unlink removes the mark and keeps the text.
4. `javascript:alert(1)` and `data:text/html,x` are refused with a visible
message; no link is created AND nothing is written to the mark.
5. `example.com` is stored as `https://example.com`.
6. Pasting `https://example.com` over a selection wraps it rather than replacing
it.
7. The horizontal-rule command inserts a break serializing to `---`.
8. Undo and redo buttons work and are **`aria-disabled`** (never natively
`disabled`) at the ends of the history stack, with activation refused while in
that state.
9. Opening an entity that contains a link and saving it without edits emits
nothing (the write-back guard still holds).
10. A link can be inserted, retargeted AND unlinked without a mouse.
11. Selecting across an existing link and pressing the link button retargets it
— it never destroys the link or discards the entered URL.
12. A pre-existing `javascript:` link in a body round-trips unchanged when an
unrelated part of that body is edited.
13. `mailto:a@b.com?bcc=x@y.com` is stored as `mailto:a@b.com`, and the user is
told the parameters were dropped.
14. `example.com:8080/path` is stored as `https://example.com:8080/path`.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the open question (adopt a packaged tooltip or build
one) was settled by reading the installed package source directly; see below.

**Existing Solutions:**

*The packaged link tooltip exists, and we are deliberately not using it.*
`linkTooltipPlugin` ships at `@milkdown/kit/component/link-tooltip` (re-exported
from `@milkdown/components/link-tooltip`), together with `configureLinkTooltip`,
`linkTooltipAPI` and `linkTooltipConfig`. Its strings are configurable and
mostly glyphs, so i18n was not the blocker.

**Decision: build locally.** Three reasons, all verified in the package source:

1. **It registers a COLLIDING `$command('ToggleLink', …)`**
(`components/src/link-tooltip/command.ts`) alongside commonmark's. These do not
override each other: `createCmdKey` mints a fresh `Symbol` per slice, and string
lookup in `@milkdown/ctx/src/context/container.ts:19` is
`[...sliceMap.values()].find(x => x.type.name === slice)` — **first registered
wins, silently**. With both installed, `callCommand('ToggleLink')` would hit
whichever registered first. Our `commandNamesExistInEditor` test would NOT catch
this: the name exists, it is simply the wrong command. Since this repo's whole
convention is calling commands by string name (`editorCommands.ts` module
header), adopting the package means permanently exempting one command from that
convention.
2. **It ships NO CSS.** Zero `.css` files, no `./style` export. We would still
write the positioning, the `[data-show='false'] { display: none }` rule, and all
`.milkdown-link-preview` / `.link-edit` chrome ourselves — so the styling saving
is much smaller than it appears, while the `.md-body` contract risk stays.
3. **Its empty-selection confirm inserts the raw URL as its own text and marks
it**, bypassing our `normalizeLinkUrl` gate. Reconciling that with the allowlist
means intercepting its API anyway.

It also pulls `@floating-ui/dom`, `dompurify`, `lodash-es` and `nanoid` into
this path. Building locally follows the `SlashProvider` pattern already in
`MilkdownEditor.vue` and keeps one way to call a command.

*Everything else needed already ships in the preset.* Verified by reading
`node_modules/@milkdown/preset-commonmark/lib/index.js`:

- `ToggleLink` — `:376`, `$command("ToggleLink", ctx => (payload = {}) =>
toggleMark(linkSchema.type(ctx), payload))`. Because it is `toggleMark`, an
empty payload over an existing link REMOVES it, so **unlink needs no new
command**.
- `UpdateLink` — `src/mark/link.ts:95`, payload
`{ href?: string; title?: string }`. Retargets the link under the cursor and
returns `false` when the cursor is not on one, so its dry run IS meaningful
(unlike `InsertHr`). Note it rewrites the mark over the matched node's **full
`nodeSize`**, not just the selection, and collapses the selection to `$anchor` —
correct for "edit this link's href", but it means a partial selection inside a
link still retargets the whole link.
- `InsertHr` — `src/node/hr.ts:64`, no payload. The schema node is **`hr`**;
`thematicBreak` is the mdast name only, so an `ActiveProbe` must say `hr`.
Caveat: it returns `true` even on the `!dispatch` dry-run path, so
`commandAvailability.ts` will ALWAYS report it available. That is acceptable
(inserting a rule is valid nearly everywhere) but it means the dry run proves
nothing here — do not read an availability pass as evidence the command works.
- The link mark's attr is **`href`**, not `url` (`:328`). The mdast node uses
`url`; the ProseMirror mark uses `href`. Mixing these up is the most likely
silent bug in this ticket.
- Undo/redo are `Undo` / `Redo` (`@milkdown/plugin-history`), and
`@milkdown/prose/lib/history.js` is `export * from 'prosemirror-history'`, so
`undoDepth` / `redoDepth` are importable from `@milkdown/kit/prose/history` for
the disabled state.

*Reusable local patterns.* The floating surface follows the `@` mention menu
(`useMentionMenu.ts` + `MentionMenu.vue` + a `SlashProvider`), which is the
closest existing shape for a positioned panel driven by editor state. Arbitrary
user input inserted via a direct `view.dispatch` follows `insertRefAtCursor`
(`MilkdownEditor.vue:363-377`). A new toolbar entry is an `EditorCommand`
descriptor plus a `BlockIcon.vue` glyph case; the toolbar's `v-for` picks it up.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

No new dependencies. Five pieces:

1. **`linkUrl.ts` (new, pure).** `normalizeLinkUrl(input): {ok: true, url} |
{ok: false, reason}`.

**Do NOT delegate normalization to the preset's `sanitizeLinkHref`.** An earlier
draft of this plan did. Measured against the installed package, that is wrong:
`sanitizeLinkHref` normalizes only to DECIDE the scheme, then returns the
original `trimmed` string. So `sanitizeLinkHref('https://ex.com/<ZW>path')`
returns the value *with* the zero-width character still in it, and a leading
zero-width is not even refused. Those bytes would then serialize into the entity
file — the exact threat this gate exists to stop. (It does fail closed on
obfuscated *schemes*: `ja<ZW>vascript:` is refused. But relying on that is
relying on a property the function does not advertise.)

**The algorithm, fully specified and measured** (24 cases run against the real
`URL` parser; table below). Order matters:

1. Strip the ignored characters ourselves (C0/C1 and space, `0x7f`-`0xa0`,
zero-width `0x200b`-`0x200d`, `0x2028`, `0x2029`, BOM), then trim. Reject empty.
2. Reject anything starting `//` — protocol-relative inherits the page
scheme and reads like a path while pointing off-origin.
3. **Test the bare-host pattern BEFORE the scheme pattern.** A dotted host
with an optional numeric port: `^HOST(\.HOST)+(:\d{1,5})?(\/|\?|#|$)` where
`HOST = [a-z0-9]([a-z0-9-]*[a-z0-9])?`. If it matches, prepend `https://`. Order
is load-bearing: `example.com:8080/path` otherwise matches the scheme pattern as
a bogus `example.com:` scheme and is refused — a common paste failing with a
nonsense message.
4. Else if it has a real scheme (`^[a-z][a-z0-9+.-]*:`), keep as-is.
5. Else refuse — this is what rejects `/relative/path`, `../up`,
`user@example.com` and bare words, per the no-relative-paths policy.
6. Parse with `new URL()`; refuse on throw. **Store `url.href`**, the
normalized form — this is what removes the embedded junk.
7. Allowlist `url.protocol` against `http:`, `https:`, `mailto:` ONLY.
8. **`mailto:` loses its query string.** `?bcc=`/`?body=`/`?subject=` is a
phishing primitive (a link reading "email support" that silently BCCs an
attacker), nobody hand-authors it in an entity body, and the repo already treats
mail header injection as a real threat — `internal/mail` rejects CR/LF in every
caller-supplied header value. Being laxer here than in the mailer would be
inconsistent. Set `u.search = ''` and tell the user.

Measured behaviour (all verified, not predicted):

| Input | Result |
|---|---|
| `example.com` | `https://example.com/` |
| `example.com:8080/path` | `https://example.com:8080/path` |
| `https://ex.com/<ZW>path` | `https://ex.com/path` (junk removed) |
| `<ZW>https://ex.com/a` | `https://ex.com/a` |
| `mailto:a@b.com?bcc=evil@x.com` | `mailto:a@b.com` (query stripped) |
| `MAILTO:A@B.com` | `mailto:A@B.com` |
| `javascript:` / `ja<ZW>vascript:` | refused |
| `data:` / `tel:` / `ftp:` | refused |
| `/relative/path`, `../up`, `notaurl` | refused |
| `//evil.com/x` | refused (protocol-relative) |
| `localhost:3000` | refused (no dot; acceptable) |

Pure and unit-tested without mounting, like `mentionQuery.ts`. Tests must pin
`tel:` and `ftp:` as refused (the two the preset allows and we do not), and must
assert the RETURNED string contains no ignored characters — not merely that bad
input is refused, which passes today for the wrong reason.

**Do not import `sanitizeLinkHref` at all.** Checked: it is exported from the JS
but does NOT appear in the package's public `.d.ts` surface, so importing it
costs a `@ts-expect-error` or a deep path into `lib/`. Since step 1 now strips
the ignored characters itself and step 6 normalizes via `new URL()`, the
preset's sanitizer adds nothing on the write side — it remains our defence in
depth on the RENDER side, where the preset calls it for us. This removes the
dependency question entirely.

2. **Toolbar commands** in `editorCommands.ts`: a `link` entry in
`INLINE_COMMANDS` (`ToggleLink`, probe `{kind: 'mark', mark: 'link'}`) and an
`hr` entry in `BLOCK_COMMANDS` (`InsertHr`, probe `{kind: 'none'}`). Plus glyph
cases in `BlockIcon.vue` keyed on those ids. Slice names as STRINGS per the
module header.

3. **Link dialog.** The `link` toolbar button does not dispatch `ToggleLink`
directly — it needs a URL first. On a collapsed selection it also needs link
text. Reuse the `EntityPickerModal` shape: a small Vue component outside the
editable div, inserting through a direct `view.dispatch`, with the same
refocus-after-dispatch discipline as `insertRefAtCursor`. Register it with
`useModalStack` (see Accessibility).

**A selection that overlaps an existing link must RETARGET, never
`ToggleLink`.** This is a data-loss defect, verified in
`node_modules/prosemirror-commands/dist/index.js:700-701`: `toggleMark` defaults
`removeWhenPresent: true` and computes `add = !ranges.some(r =>
doc.rangeHasMark(...))` — **`.some`, not `.every`**. So if ANY part of the
selection already carries a link, the command takes the REMOVE branch. A user
selecting across `see [docs](url) here` and pressing the link button would
destroy the existing link and silently discard the URL they just typed,
discovering it only at save.

Required behaviour: before opening the dialog, test whether the selection
intersects a `link` mark. If it does, prefill the dialog with that link's href
and commit through `UpdateLink` — retargeting the WHOLE link, which is what
`UpdateLink` does anyway (it rewrites over the matched node's full `nodeSize`,
so a partial selection cannot produce a partial retarget). Only a selection with
no link at all takes the `ToggleLink` insert path.

This also gives the toolbar button its context-sensitive behaviour: caret inside
a link → edit that link. That is the keyboard route to retarget, and it is why
the tooltip need not be the only path (see Accessibility).

4. **Link tooltip — hover AND cursor.** Built locally on
`TooltipProvider` from `@milkdown/kit/plugin/tooltip`, which is already a
dependency and is the same floating-ui shape as the `SlashProvider` this file
uses for `@` (`{ content, debounce, shouldShow }` + `update`/`hide`/`destroy`,
`data-show` toggling, appended outside the editable div). Follow the existing
`slashProvider` lifecycle: module-level ref, constructed in `view`, `destroy()`
on teardown.

Two trigger paths into ONE panel:
- *Cursor* — caret inside a `link` mark. This is the keyboard-reachable
path and the one the tests assert.
- *Hover* — pointer over a link, debounced (~50ms). Discoverability only;
it must never be the sole way to reach an action.

`TooltipProvider.show()` accepts an explicit `VirtualElement`, which is how a
hover anchors to the hovered mark rather than to the selection — its default
`shouldShow` requires a non-empty selection, so it must be overridden for both
paths.

Rules that keep the two from fighting: cursor wins over hover when both are live
(never two panels); hovering away hides, but a caret-anchored panel stays until
the caret leaves. It must not appear over an `entityRef`, which is a different
construct.

**Escape needs dismissal MEMORY, not just a close call — follow
`dismissedQuery`.** Closing a provider-driven surface from a key handler does
nothing on its own: `shouldShow` runs on the next update, the caret is still
inside the link, and the panel immediately reopens, so Escape looks broken. The
`@` menu already hit this and solved it with `dismissedQuery`
(`MilkdownEditor.vue:254-262`). The link panel needs the equivalent — a
`dismissedLinkPos` (or mark identity) consulted by `shouldShow` and cleared when
the caret leaves that link.

This also means `onKeydownCapture` must change: it currently returns immediately
unless the mention menu is open (`MilkdownEditor.vue:453`), so it would never
see Escape for the link panel. State the precedence — mention menu first (it is
modal-ish and transient), then link panel — so a stray overlap resolves one way
rather than both. `MilkdownEditor.vue` is therefore NOT merely "wiring" in the
file list.

Edit reopens the dialog prefilled and commits via `UpdateLink`; Unlink
dispatches `ToggleLink` with an empty payload.

**The tooltip reads SELECTION and pointer state only — it must never dispatch a
document transaction**, or it trips the `armed` write-back guard and writes a
diff for an entity that was merely opened.

5. **Paste handling.** A `$prose` plugin with `handlePaste`. Return `true`
(swallowing the paste) ONLY when every one of these holds; otherwise return
`false` and let ProseMirror behave normally:

   - the selection is non-empty,
   - `event.clipboardData.getData('text/plain')` normalizes to an allowed URL
via the SAME `normalizeLinkUrl` (read `text/plain` explicitly — a browser paste
often carries `text/html` too, which ProseMirror prefers, and using it here
would mean pasting rich content instead of a URL),
   - the selection does not already carry a link mark (else it is a retarget,
which belongs to the dialog path — see Approach 3),
   - the mark can actually apply: refuse inside `code_block`, whose schema is
`marks: ''`. **Check this BEFORE returning `true`**, or the paste is swallowed
and nothing is inserted.

Registration order matters. `.use()` order in `MilkdownEditor.vue:594-610` is
already load-bearing (there is a comment pinning `taskList` after `gfm`); state
that `linkPaste` registers after the presets so its `handlePaste` sees the event
first.

A selection spanning two blocks yields two separate links with the same href on
save — this is inherent to applying an inline mark across a block boundary.
Assert the serialized result is sane rather than pretending it is one link.

6. **Undo/redo buttons — reuse the descriptor machinery, do not build a
parallel channel.** An earlier draft put them outside `EditorCommand` because
they "must not be probed for active state". That is already expressible: `probe:
{kind: 'none'}` means exactly "never active". Commands are `Undo` and `Redo`
(`@milkdown/plugin-history`).

Availability is the only part that does not fit the dry run, and there is an
established extension point for precisely that — the `canRunTableCommand` /
`canAddRowBefore` overrides at `MilkdownEditor.vue:296-304`, which exist because
the dry run is untrustworthy for tables. `undoDepth(state) === 0` /
`redoDepth(state) === 0` is the same shape of override, computed in the existing
`refreshDerivedState` path and folded into `unavailableIds`.

Doing it this way inherits `EditorToolbar.vue`'s **`aria-disabled`** handling
and its `onActivate` refusal guard for free. That matters more here than
anywhere else in the toolbar: undo/redo availability flips on *every*
transaction, so a natively `disabled` button would drop focus to `<body>` the
instant a user tabbed to it starts typing — the exact bug
`EditorToolbar.vue:35-43` documents. `undoDepth`/`redoDepth` come from
`@milkdown/kit/prose/history` (the plugin does not re-export them).

*Alternative rejected:* adopting `@milkdown/kit/component/link-tooltip` — it
exists, but collides on the `ToggleLink` slice name, ships no CSS, and bypasses
our URL gate on empty-selection confirm (see Research). *Alternative rejected:*
relying on the preset's `sanitizeLinkHref` alone instead of write-side
validation — see Security.

**Files to modify:**

- `frontend/src/components/forms/milkdown/linkUrl.ts` (new)
- `frontend/src/components/forms/milkdown/linkUrl.test.ts` (new)
- `frontend/src/components/forms/milkdown/LinkDialog.vue` (new)
- `frontend/src/components/forms/milkdown/LinkTooltip.vue` (new)
- `frontend/src/components/forms/milkdown/linkPaste.ts` (new, `$prose` plugin)
- `frontend/src/components/forms/milkdown/editorCommands.ts` (link + hr entries)
- `frontend/src/components/forms/milkdown/BlockIcon.vue` (glyphs)
- `frontend/src/components/forms/milkdown/EditorToolbar.vue` (undo/redo group)
- `frontend/src/components/forms/milkdown/MilkdownEditor.vue` (wiring — see
the size constraint below)
- `frontend/src/components/forms/milkdown/useLinkUI.ts` (new composable holding
the dialog and tooltip open/close state and the commit/unlink calls)
- `frontend/src/components/forms/milkdown/milkdownEditor.css` (tooltip chrome)
- `e2e/tests/markdown-editor-links.spec.ts` (new) and `e2e/pages/form.page.ts`
(locators)

**Size constraint — extract, do not grow.** `MilkdownEditor.vue` is already 843
lines against the repo's 500-line `max-lines` warning (`eslint.config.ts:118`),
and `frontend/CLAUDE.md` calls that rule the god-component catch. Three new
surfaces' worth of state would push it past 1000. So the dialog/tooltip state
goes in `useLinkUI.ts`, following how `useMentionMenu.ts` already keeps the `@`
menu's state out of the component; `MilkdownEditor.vue` gains only the `.use()`
registration and the wiring props. This is a requirement of the plan, not a
nice-to-have — an implementation that inlines it has to be sent back.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

One untrusted input: the URL, typed into the dialog or arriving via paste.
Validated by `normalizeLinkUrl` with a strict **allowlist** (`http:`, `https:`,
`mailto:`). Invalid input is refused with a visible message and NO mark is
written — never silently corrected to something else.

**Why write-side validation is required and is not redundant.** The preset's
`sanitizeLinkHref` (`preset-commonmark/src/mark/sanitize-href.ts`) runs at three
DISPLAY sinks — the mark's `toDOM`, the preview anchor, and the packaged
tooltip's confirm. All of them are render-time. It blanks the rendered `href`,
but the original string stays in the ProseMirror mark and **still serializes
into the markdown file**. A `javascript:` URL would therefore be persisted to
the entity body and handed to every other renderer of that content, several of
which are outside this editor's control.

Its allowlist is also wider than ours (`http`, `https`, `mailto`, `tel`, `ftp`)
and it returns any scheme-less string unchanged — relative paths, `#frag` and
`?q=` all pass. So the editor must validate on the way IN. We REUSE its
character-stripping (see Approach 1) and narrow its allowlist; we do not rely on
it as the gate. It remains defence in depth on the way out.

`rawHtmlPassthrough.test.ts:91` already pins the render-side half.

**The gate is on NEW input only — never on load. This boundary is pinned by an
existing test and must not be crossed.** `normalizeLinkUrl` runs where a user
supplies a URL: the dialog (typed or edited) and the paste plugin. It must NOT
run at the markdown-parse step on document load.

A body that already contains `[click](javascript:alert(1))` keeps that text.
That is deliberate, not an oversight: `rawHtmlPassthrough.test.ts:94` asserts
such a link never renders as a navigable href (the preset blanks it at `toDOM`),
while `:109` asserts the stored bytes round-trip UNCHANGED, with the comment
that sanitizing at parse would "silently rewrite the author's stored body — a
data-integrity bug wearing a security fix's clothes." Cleaning existing bodies
is a migration, not an editor change.

So: unsafe link already in the file → not rendered clickable, not rewritten.
Unsafe link the user tries to ADD → refused. Both paths into the mark (dialog
and paste) go through the same `normalizeLinkUrl`; a test should assert the
paste path cannot bypass it, since that is the easy one to forget.

**The consequence, stated plainly so nobody believes more is covered than is.**
A pre-existing `javascript:` URL is parsed into the mark unvalidated, and if the
user edits anything else in that body, `guardWriteBack` returns `edited` and the
editor writes the body back out **with the hostile URL preserved**. The editor
therefore still *carries* a URL it would never *accept*. That asymmetry is
deliberate, and it is the correct trade here:

- Rewriting on load would break the losslessness contract that
`rawHtmlPassthrough.test.ts:109` exists to protect, silently altering an
author's stored bytes.
- The URL is defanged at every render sink already — the preset's `toDOM` for
the editor, DOMPurify for the rendered view — so it is inert, not live.
- Cleaning existing bodies is a MIGRATION (operator-run, auditable, reversible
by review), not an editor behaviour.

**This ticket closes the insert half of the surface, not the whole surface.**
Pin the decision with a test asserting a pre-existing `javascript:` link
round-trips UNCHANGED, mirroring the `rawHtmlPassthrough` precedent — so the
behaviour is a recorded choice rather than an accident a later reader "fixes"
into a data-integrity bug.

**Security-Sensitive Operations:**

- Writing a `link` mark → gated by `normalizeLinkUrl`.
- No file access, auth or crypto in this change.
- Refusal messages echo only a fixed reason, never the offending string, so a
crafted URL cannot inject markup into the error surface.

**`mailto:` is accepted with a known, accepted residual risk.** It round-trips
cleanly (measured: `mailto:a@b.com?subject=hi&body=x`, `?bcc=evil@x.com` and a
`%0A`-encoded variant all serialize and reparse byte-exact), so there is no
serialization or injection defect. The residual risk is social: a `mailto:`
carrying hidden `bcc=`/`body=` parameters can render as innocuous link text, so
a reader clicking it may send more than they expect. That is inherent to
`mailto:` as a scheme — the preset allowlists it for the same reason — and the
entity body is already operator/author-authored content in this app. Accepted,
not overlooked. If it ever needs tightening, the place is `normalizeLinkUrl`,
which is precisely why the allowlist lives in one function.

## Accessibility

The house rules in `frontend/CLAUDE.md` are specific, and this ticket adds two
new interactive surfaces plus two disable-able buttons, so they all apply.

- **`aria-disabled`, never native `disabled`, on the undo/redo buttons.** The
existing toolbar already does this and documents why
(`EditorToolbar.vue:35-43`): a natively disabled control cannot be focused, so
disabling the button a user is tabbed to drops focus to `<body>`
mid-interaction. Undo/redo flip between enabled and disabled constantly, which
makes them the worst case for that bug. Since `aria-disabled` does not prevent
activation, the handler must also refuse — matching `onActivate`.
- **Focus rings use the two-shadow token pattern**, not a hand-written colour:
`0 0 0 2px var(--focus-ring-gap), 0 0 0 4px var(--focus-ring)`. Applies to the
dialog inputs and every button in the tooltip.
- **A scoped `<style>` needs its own `prefers-reduced-motion` rule** if anything
animates; a scoped class compiles to `[data-v-*]` and outranks the unscoped rule
in `pending.css`, so adding it there would silently do nothing.
- **Hover is not keyboard reachable, so it must never be the only route — and
"the panel appears" is NOT by itself a keyboard route.** The panel is appended
outside the editable div, so `Tab` from inside ProseMirror does not reach it in
any useful order. Without an explicit affordance, Edit and Unlink would be
mouse-only, failing WCAG 2.1.1 and this plan's own rule. Since `Mod-k` is
descoped, the keyboard route is **the toolbar link button, made
context-sensitive**: with the caret inside a link it opens the dialog prefilled
for retarget (which the overlap rule in Approach 3 requires anyway), so insert
and retarget are both reachable by keyboard through a control that is already in
the tab order.

**Unlink still needs its own keyboard-reachable control.** The toolbar button
cannot be both "edit this link" and "remove this link". Options: a second
toolbar button shown only when the caret is in a link, or making the link button
a toggle whose active state removes. Decide during implementation of the toolbar
group, but an AC must assert unlink is reachable without a mouse.
- **The dialog is a modal**: focus moves into it on open, is trapped while open,
Escape closes it, and focus returns to the editor at the caret. Reuse
`EntityPickerModal`'s handling rather than reinventing it. **Register it with
`useModalStack(open)` from `composables/modalStack.ts`** — one call wires the
reactive `open` ref in and cleans up on unmount. This is not optional
bookkeeping: `useKeyboardShortcuts.ts:55` stands down while `isAnyModalOpen()`,
and its comment explains the guard exists so Escape is not double-handled and
does not trigger `router.back()` on a form page underneath. An unregistered
dialog means Escape closes the dialog AND navigates away, losing the user's
edits.
- **The tooltip is not a modal** and must not trap focus; it is reachable in the
tab order while the caret is in the link, and Escape dismisses it.
- **Announce the refusal.** A URL rejection is a validation error: use
`--error-ring` on the field and put the message in a live region, or a screen
reader user gets no feedback at all.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
|---|---|
| 1 | Unit (mount): select text, run link command with a URL, assert `__relaGetMarkdown` yields `[text](https://example.com)` |
| 2 | Unit (mount): place cursor in a link, assert tooltip visible and shows the href. Hover path asserted in e2e, where a real pointer exists |
| 3 | Unit (mount): unlink, assert mark gone and text retained |
| 4 | Unit (pure) on `normalizeLinkUrl` for each refused scheme + mount test asserting no mark written |
| 5 | Unit (pure): `example.com` → `https://example.com` |
| 6 | Unit (mount): synthesize a paste event over a selection, assert the mark is applied and the text is not replaced. Constructible — `DataTransfer`, `ClipboardEvent` and `PointerEvent` are all present in happy-dom (checked) |
| 7 | Unit (mount): run hr command, assert markdown contains `---` |
| 8 | Unit (mount): assert buttons disabled at stack ends, enabled after an edit |
| 9 | Corpus/guard test: load a body with a link, save untouched, assert no emit |

**The unit environment is `happy-dom`** (`vitest.config.ts:25`), not jsdom.
`DataTransfer`, `ClipboardEvent` and `PointerEvent` are all constructible there
(verified), so the paste test is a unit test. What happy-dom still cannot do is
LAYOUT: `getBoundingClientRect` returns zeros, which floating-ui consumes, so
tooltip POSITION cannot be asserted in a unit test. Follow the house precedent
in `frontend/CLAUDE.md` — assert the structural preconditions in units (panel
present, `data-show` toggled, href text rendered, buttons wired) and leave
pixels and real hover to e2e.

E2E (`e2e/tests/markdown-editor-links.spec.ts`): insert a link through the real
toolbar, reload the entity, confirm it persisted; unlink and confirm removal;
**hover a link and assert the panel appears**, which needs a real pointer. This
is the integration half — the unit tests mount the editor but do not prove the
value survives a server round-trip.

**Edge Cases:**

- Empty URL input → refused, no mark.
- Whitespace-only URL → refused.
- Collapsed selection → dialog must ask for link text; empty text refused.
- Selection spanning an existing link → retarget, do not nest marks.
- Selection spanning a link and plain text → defined behaviour asserted.
- Selection containing an `entityRef` atom node → must not wrap it.
- **Link inside a code block → refused BY THE SCHEMA, no special case needed.**
`codeBlockSchema` declares `marks: ''`
(`preset-commonmark/src/node/code-block.ts:31`), so ProseMirror rejects a link
mark there and the `commandAvailability.ts` dry run disables the button on its
own. Assert it; do not implement it.
- Selection spanning multiple blocks (e.g. two paragraphs) → `toggleMark`
applies per text node; assert the serialized result is sane rather than one link
swallowing the block boundary.
- Link inside a table cell → must work; tables are the other place marks
behave unusually, and `tableCommands.ts` documents that the GFM table schema
already surprises the dry run.
- URL with query + fragment → preserved intact.
- **IDN / homograph hosts → accepted, by decision.** `https://пример.рф/путь`
round-trips unchanged and `https://еxample.com` (Cyrillic е) is accepted.
Homograph detection is explicitly NOT attempted: the browser's address bar is
the trust boundary for that, and punycode-normalizing would surprise legitimate
non-ASCII users. Recorded as a decision, not an untested case.
- Unlinking `[](https://x)` — a link whose text is already empty (it does parse
as a `link` node) → must not throw; there is no text to keep.
- Link inside a table cell → works (cells allow inline marks); the untested
part is TOOLTIP POSITION inside a scrollable table container, which is an e2e
concern. `tableCommands.ts` is a documented trouble spot in this directory, so
do not assume.
- Very long URL (5000 chars) → round-trips byte-identical (measured).
- Nested marks: `**[x](url)**` and `[**x**](url)` both round-trip (measured).
- **URL containing markdown-significant characters — MEASURED, not assumed.**
A space, `)`, `(`, `&` or a newline in the href does NOT need handling in our
code: remark-stringify already angle-wraps or backslash-escapes, and all of
these round-trip byte-exact under `RELA_STRINGIFY_OPTIONS`. Verified by running
the real parse→serialize pair from `frontend/node_modules`:

  | href | serialized | reparsed |
  |---|---|---|
  | `https://example.com/a b` | `[t](<https://example.com/a b>)` | identical |
  | `https://example.com/a)b` | `[t](https://example.com/a\)b)` | identical |
  | `https://example.com/a(b)c` | `[t](https://example.com/a\(b\)c)` | identical |
  | `https://ex.com/?q=1&x=2#f` | `[t](https://ex.com/?q=1\&x=2#f)` | identical |

So the ticket does NOT need escaping logic. Keep a test pinning this, because it
is a property of remark's config we depend on rather than one we enforce.
- `MAILTO:` uppercase scheme → accepted case-insensitively.
- `java script:` / tab-and-newline-obfuscated scheme → refused.
- Paste of a URL with an EMPTY selection → falls through to normal paste
(plain text), does not create an empty link.
- Paste of non-URL text over a selection → normal replace.
- Undo immediately after link insert → restores prior state in ONE step.
- Hover a link while the caret sits in a DIFFERENT link → one panel, cursor
wins.
- Hover away from a caret-anchored panel → panel stays (caret still owns it).
- Escape with the panel open → dismisses without touching the document.
- Hover an `entityRef` → no link panel.

**Negative Tests:**

`javascript:`, `data:`, `vbscript:`, `file:`, `tel:`, `ftp:` (the last two are
allowed by the preset but NOT by our policy — these pin the narrowing),
scheme-less relative paths (`/foo`, `../bar`), and obfuscated variants. Each
must be refused with a visible message and leave the document unchanged.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *`href` vs `url` confusion.* The ProseMirror mark attr is `href`; mdast uses
`url`; `serializerContract.ts:82` lists `link: ['url', 'title']` for the mdast
side. Using the wrong one yields a link with an undefined target that still
looks right in the editor. Mitigated by a round-trip assertion **plus a separate
assertion on the href the TOOLTIP displays** — a `__relaGetMarkdown` round-trip
would pass even if the tooltip read the wrong attribute, because that is a
different read path.
- *Write-back guard regression.* Any new transaction on load could trip the
`armed` flag and write a diff for an entity only opened. The tooltip must be
driven by SELECTION state, never by a document transaction. AC 9 plus the corpus
test cover this.
- *Paste plugin over-reach.* `handlePaste` returning `true` too eagerly breaks
ordinary pasting. Mitigated by the narrow condition (non-empty selection,
clipboard text normalizes to an allowed URL) and the fall-through tests.
- *Tooltip positioning.* Built locally on `TooltipProvider`, the same
floating-ui shape as the existing `SlashProvider`; follow that usage.
- *`ToggleLink` slice collision (latent).* Recorded here because the trap
survives this ticket: if anyone later adds
`@milkdown/kit/component/link-tooltip`, its `$command('ToggleLink', …)` joins
the container alongside commonmark's and string lookup silently resolves to
whichever registered first. `commandNamesExistInEditor` cannot detect it. A
comment at the `link` command descriptor should say so.
- *Hover/cursor interaction.* Two triggers into one panel is where the fiddly
bugs live (flicker, two panels, a panel that will not dismiss). Mitigated by the
precedence rules in Approach 4 and by asserting the cursor path in unit tests,
hover in e2e where a real pointer exists.

**Effort:** m (unchanged). The tooltip must be hand-built, but that is offset by
`ToggleLink`/`UpdateLink`/`InsertHr` all existing and by the shortcut work
dropping out of scope.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` § "The Markdown Body Editor" — **line 441 enumerates
the supported constructs** ("headings, lists, tables, quotes and code blocks")
and must gain links and horizontal rules, or the list becomes wrong the day this
ships. Also state the accepted URL schemes there: a refused `tel:` or relative
link is otherwise a silent surprise with no documented explanation.
- [x] `frontend/CLAUDE.md` — the markdown-editor section should record that URL
validation is write-side and why the preset's sanitizer does not suffice.
- [x] ~~Other docs~~ (N/A: no metamodel, CLI, README or schema surface changes —
this is a data-entry UI change only).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 19 findings (3 critical, 7 significant, 6 minor, 3
leverage). All critical and significant are resolved in this plan. The three
critical ones are tracked as review-responses: **RR-VR84MY** (sanitizer returns
unnormalized bytes), **RR-K0PGQW** (`ToggleLink` destroys an overlapping link),
**RR-Z5WLG1** (load path preserves hostile URLs; decision was unstated). All
three are `addressed`.

Critical, all three verified against installed source rather than accepted on
assertion:

1. **`sanitizeLinkHref` returns UNNORMALIZED bytes.** Measured: it normalizes
only to decide the scheme, then returns the original string — so a zero-width
character inside a path survives into the stored markdown, and a leading one is
not even refused. The plan's original "compose the preset's sanitizer" mechanism
did not do what it claimed. Fixed: we strip the ignored characters ourselves and
normalize through `new URL()`, storing `url.href`. It fails closed either way
(no XSS bypass existed), but the plan was relying on a property the function
does not have.
2. **`ToggleLink` DESTROYS an overlapping link.** Verified at
`prosemirror-commands/dist/index.js:700-701` — `removeWhenPresent` defaults true
and tests `.some()`, not `.every()`. Selecting across an existing link and
pressing the button would have removed it and silently discarded the typed URL,
discovered only at save. Fixed by the retarget rule in Approach 3 and pinned by
AC 11.
3. **The load path was an unstated gap.** Now stated explicitly, with the
reasoning for NOT sanitizing on load and a test pinning the decision.

Significant, all resolved: the `https://` prepend heuristic is now a written,
measured algorithm (was "looks like a host"); `mailto:` query stripping decided;
keyboard reachability given a concrete affordance; Escape given dismissal memory
following `dismissedQuery`; paste plugin given its ordering, clipboard type and
code-block guard; undo/redo folded into the existing descriptor +
`aria-disabled` machinery; test environment corrected to happy-dom (and the
paste APIs confirmed constructible there).

Three findings were checked and REJECTED as already-correct: the
markdown-significant-character worry (remark handles it — measured), the
write-back guard interaction (selection-driven panels cannot arm `dirty`), and
the code-block case (schema `marks: ''` already disables it via the dry run).
