---
id: RR-IKYS9
type: review-response
title: Renderer fallback for legacy {task, checked, text} list items unspecified
finding: Plan says list-item tables get inlines instead of text, and constructors auto-wrap strings. But renderer behavior when a script supplies {task=true, text='foo'} (no inlines) isn't specified — silent ignore? Auto-wrap on render? Error? Pick auto-wrap (script-friendly) since constructors already do it.
severity: minor
resolution: 'Renderer fallback documented in Decisions: if a list-item or block table lacks `inlines` but has `text` (string), wrap-on-the-fly to `{{type=''text'', text=s}}` for rendering only. Parse-produced tables always have `inlines`.'
status: addressed
---

# Finding

Current renderer (`renderListItemTable` at `markdown.go:994-1010`) reads `text`
from the item table.

After refactor, items have `inlines`. Scripts that construct items by hand might
still pass `{task=true, text="foo"}` (no `inlines`) expecting it to render. Plan
says constructors auto-wrap, but that's at construction, not render.

# Resolution

Renderer fallback policy: if a list-item table has `inlines`, use it. Else if it
has `text` (string), render as if it were `{{type="text", text=s}}`. Else render
empty.

This matches the construction-time auto-wrap behavior and gives hand-written
tables a graceful path. Document the fallback: it's a deliberate compatibility
shim for hand-written items, NOT for parse-produced items (which always have
`inlines`).

Add tests:

- `{rela.md.list({{task=true, text="hi"}})}` renders to
`- [x] hi\n` (auto-wrap on render).
- `{rela.md.list({{task=true, inlines={{type="text",text="hi"}}}})}`
renders identically.
