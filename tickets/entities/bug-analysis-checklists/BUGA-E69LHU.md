---
id: BUGA-E69LHU
type: bug-analysis-checklist
title: 'Analysis: Milkdown editor renders GFM task lists as bullets with no checkbox'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Reproduced by mounting the real component in the existing
`MilkdownEditor.test.ts` harness with `modelValue: '- [ ] todo\n- [x] done\n'`
and dumping the ProseMirror DOM. The editor emits:

```html
<ul data-spread="false">
  <li data-item-type="task" data-checked="false"><p>todo</p></li>
  <li data-item-type="task" data-checked="true"><p>done</p></li>
</ul>
```

No `<input>`, so the two items are visually identical bullets. The document
state is correct (`list_item` attrs carry `checked: false` / `checked: true`)
and the write-back verdict is `unchanged`, confirming the defect is purely in
rendering and interaction, not in markdown handling.

Environment: `@milkdown/kit` 7.22.1, vitest 4 + jsdom, `frontend/`.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

Recorded in `why1`-`why5` on the bug. In short: the GFM preset leaves the
checkbox affordance to the host app, and the port relied on the shared
`.md-body` stylesheet, whose task-list rules key on `input[type='checkbox']` —
an element Milkdown never emits. The corpus round-trip gate stayed green because
serialization was never broken.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

### Approach

Three parts, in `frontend/src/components/forms/milkdown/`.

**1. Render a real checkbox via a node view.** Rather than styling
`li[data-item-type='task']`, attach a ProseMirror node view for `list_item` that
injects an actual `<input type="checkbox">` as the first child when `checked !=
null`, with `contenteditable="false"` on the input and the item text in a
`contentDOM` sibling. This makes the editor's DOM structurally match what
`marked` produces, so the existing `.md-body` rules in
`markdown-content.css:122-145` apply unchanged and the two surfaces stay coupled
through one stylesheet rather than two parallel rule sets. It also gives a place
to hang the click handler. There is no existing node-view pattern in this
directory, so this introduces one.

**2. Click-to-toggle.** The node view's input dispatches a transaction doing
`setNodeMarkup` on the item with `checked` flipped. The input must be
`contenteditable="false"` and the handler must `preventDefault` so ProseMirror
does not try to place a cursor inside it. This mirrors the read-only view's
behaviour (`EntityDetail.vue` delegated handler), but stays local to the
document instead of issuing a PATCH.

**3. A `taskList` command + toolbar and `/` menu entry.** The preset exports
`wrapInTaskListInputRule` but **no command**, so one has to be written: a
`$command` that walks up to the enclosing `list_item` and sets `checked` to
`false` when it is `null`, or back to `null` when it is set. Register it in
`BLOCK_COMMANDS` with an icon in `BlockIcon.vue`.

The three-state attribute needs care. `checked` is `null` (not a task), `false`
(open) or `true` (done), but `isNodeActive` in `activeFormats.ts:53-57` does
exact equality, so a probe of `{ checked: false }` reports *inactive* for a done
item — the button would unpress when you tick the box. Either extend
`ActiveProbe` with a predicate form, or probe on `data-item-type` semantics. The
`toggleTo` model also assumes two states, so the inverse belongs inside the
command rather than in a `toggleTo` slice name.

### Regression tests

- Live-editor DOM test: mounting `- [ ] a\n- [x] b\n` renders two
`input[type='checkbox']`, the second `checked`. This is the test that would have
caught the bug, and it asserts the editor/marked DOM parity that `why5`
identifies as untested.
- Toggle test: clicking the input flips the emitted markdown between
`- [ ] a` and `- [x] a`.
- Command test: running `taskList` on a plain bullet emits `- [ ] `, and
running it again reverts to `- `.
- The existing `commandNamesExistInEditor` test already guards the new
command's slice name against an upstream rename.
- Serializer round-trip is already covered by `serializerContract.test.ts:77`;
no change needed there.

### Related areas checked

- **Read-only view** — correct. `marked` has `gfm: true`, emits real
checkboxes, and `checkboxToggle.ts` + `EntityDetail.vue` make them interactive
with a `(n/m)` stats widget. No change.
- **Server render (goldmark)** — unaffected; the CSS comment at
`markdown-content.css:128-129` notes both renderers place the checkbox first,
which is the shape the node view will now match.
- **Sandboxed app editor** (`src/app-editor/`, `<rela-editor>`) — still on
EasyMDE, a source-mode editor where `- [ ]` is literal text. Not affected.
- **Other GFM constructs** — tables and strikethrough are both wired
(`tableCommands.ts`, `ToggleStrikeThrough`). Footnotes are in the schema
(`footnote_definition`, `footnote_reference`) with no toolbar entry, but unlike
task lists they render visibly, so they are a missing affordance rather than a
rendering defect. Out of scope here.
- **Docs** — `docs/data-entry.md:451-455` enumerates the toolbar and will need
the new entry. It is generated; edit
`docs-project/entities/guides/GUIDE-data-entry.md`.
