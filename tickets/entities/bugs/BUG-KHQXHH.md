---
id: BUG-KHQXHH
type: bug
title: Milkdown editor renders GFM task lists as bullets with no checkbox
description: 'The Milkdown editor shows GFM task lists as plain bullets: the preset emits <li data-checked> with no <input>, so the .md-body checkbox CSS never matches. No toolbar or slash command creates one either.'
priority: medium
effort: m
why1: Milkdown's GFM task-list schema renders <li data-item-type="task" data-checked="..."> with no <input> element, while the .md-body task-list CSS the editor inherits keys entirely on input[type='checkbox'], so no selector matches and the item falls through to plain bullet styling.
why2: The GFM preset deliberately ships only a schema extension and an input rule. It leaves the visual affordance to the host app, expecting either a node view that injects a checkbox or CSS written against its data attributes. Neither was supplied.
why3: The Milkdown port (TKT-3I9DDY) treated the shared markdown stylesheet as sufficient for typography, reasoning that wearing .md-body makes the editing surface match the rendered view. That holds for every construct whose editor DOM matches marked's HTML, but task lists are the one case where the two renderers emit structurally different DOM for the same markdown.
why4: The port's acceptance gate was the serializer corpus test, which measures round-trip fidelity over 3,939 entity bodies. Task lists round-trip perfectly (checked is a meaningful key in serializerContract.ts), so the gate was green while the feature was invisible on screen. Nothing in the gate looks at rendered DOM.
why5: 'Rendering and interaction parity between the editor and the read-only view was assumed but never asserted. The two surfaces use different libraries (ProseMirror vs marked) yet share one stylesheet, coupled only by a prose comment in markdown-content.css, so a construct could render differently in one surface with every test green. The serializer corpus gate reinforced the blind spot by proving the thing that was never broken: task lists round-tripped perfectly the whole time the feature was invisible on screen.'
prevention: 'Two layers, both added. (1) An editor-DOM regression suite (measure milkdown-task-list-renders-checkbox) mounts the real editor and asserts it emits the same input[type=''checkbox''] shape the shared .md-body stylesheet selects on, turning the editor/read-only rendering coupling from a prose comment in markdown-content.css into a check. (2) Real-browser e2e coverage for anything jsdom cannot decide. The second layer was added because the review exposed the limit of the first: :has() bullet suppression and inline layout need a layout engine, and jsdom disagrees with Chromium on legacy-canceled-activation, so a jsdom-only suite passed a checkbox implementation that was broken in every real browser. The matching test-design rule: drive interactions the way a browser delivers them (mousedown THEN click, and bare click for keyboard) and exercise ranged selections, not only a collapsed cursor - that single blind spot hid both critical defects the review found.'
status: done
---

## Description

The Milkdown editor that replaced EasyMDE (TKT-3I9DDY) shows a GFM task list (`-
[ ] todo`) as an ordinary bullet. There is no checkbox to see or click, and no
way to create a task item from the toolbar or the `/` menu.

### What still works

Markdown fidelity is intact. The GFM preset IS loaded and the round trip is
lossless, verified against a live editor instance:

- `@milkdown/kit/preset/gfm` is imported at `MilkdownEditor.vue:29`.
- `serializerContract.ts:86` lists `checked` as a meaningful `listItem` key.
- Loading `- [ ] todo\n- [x] done\n` parses to `list_item` nodes carrying
`checked: false` / `checked: true`, and serializes back byte-identical with
verdict `unchanged`.

The read-only surfaces are also fine: `marked` runs with `gfm: true`
(`utils/markdown.ts:87`), emits real `<input type="checkbox">` elements, and
`utils/checkboxToggle.ts` makes them clickable in the entity body.

### What is broken

The defect is confined to the editor's rendering. Milkdown's task-list schema
extension emits a bare `<li>` with the state in a data attribute and **no
checkbox element**:

```html
<li data-item-type="task" data-checked="false"><p>todo</p></li>
```

`styles/markdown-content.css:130-145` keys all of its task-list styling on
`input[type='checkbox']`:

```css
.md-body li:has(> input[type='checkbox']:first-child) { list-style: none; ... }
.md-body input[type='checkbox'] { margin-right: 8px; cursor: pointer; }
```

The editing surface wears `.md-body`, so it inherits those rules, but none of
the selectors match a `<li data-checked>`. The item falls through to the plain
bullet style and the checked state is invisible.

So an author editing an entity that contains a checklist sees a bullet list,
cannot tell done items from open ones, and cannot toggle or create one. The
markdown survives a save, but the editor is unusable for checklists — which is a
core rela workflow, since every planning, implementation, review and docs
checklist entity is a GFM task list.

### Additionally: no way to create one

`editorCommands.ts` `BLOCK_COMMANDS` offers bullet list, numbered list, heading,
quote, code block and table, but no task list. The GFM preset ships
`wrapInTaskListInputRule` (typing `[ ] ` inside an existing list item converts
it), but it registers **no command**, so there is nothing for a toolbar button
or `/` menu entry to call — an inverse has to be written.

## Reproduction

1. Open any entity with a checklist body in the data-entry SPA, or mount the
editor with `modelValue: '- [ ] todo\n- [x] done\n'`.
2. Observe both items render as plain bullets, identical to each other.
3. No checkbox is shown; clicking where one would be does nothing.
4. The toolbar and `/` menu offer no "Task list" entry.

Verified by probing a live editor instance in the existing
`MilkdownEditor.test.ts` harness (jsdom, `@milkdown/kit` 7.22.1).
