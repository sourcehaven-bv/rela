---
id: TKT-D2JML7
type: ticket
title: Port the sandboxed app editor (<rela-editor>) from EasyMDE to Milkdown
kind: enhancement
priority: medium
effort: l
status: done
---

## Description

Port the standalone `<rela-editor>` Custom Element for sandboxed custom apps
(`frontend/src/app-editor/`) from EasyMDE to Milkdown, so the app editor and the
data-entry form run the same editor.

TKT-3I9DDY moved the SPA form to Milkdown and explicitly held this back. Two
things were left behind by that split, and both are the kind that rot:

- **A forked editor.** `relaBacktick.ts` was a 428-line hand-written CodeMirror
entity-reference autocomplete with no counterpart in the SPA, and
`relaEditorTheme.css` hand-mirrored a slice of `markdown-content.css` well
enough to need `markdownContentMirror.test.ts` to catch the copies diverging.
RR-9PTXV0 flagged exactly this and deferred the fix here.
- **A second serializer.** Two editors writing the same markdown bodies through
different code can disagree about what a body is, and nothing would say so.

### Scope

IN: the `<rela-editor>` element, its toolbar, its `@` completion, its
stylesheet, and the standalone build (`vite.editor.config.ts`) plus the Go side
that serves the result.

OUT: the element's public contract, which does not change. `value`,
`placeholder`, `readonly`, `input`, `change`, `focus()` — the swap seam exists
precisely so this move is invisible to an app, and an app must need no edit.

### The recorded CSP blocker is wrong

PLAN-JQ2FBA held this ticket back because "the app CSP carries no
`'unsafe-inline'` on `style-src`, which blocks the `style` attribute some editor
chrome wants". That premise was never tested, and it is false.

`style-src` governs stylesheets and the `style` ATTRIBUTE. ProseMirror positions
its chrome by writing DOM style PROPERTIES (`el.style.left`), which is the CSSOM
and is permitted. Verified twice: first against a probe server replicating
`appCSP` exactly (`setAttribute('style')` blocked, `style.cssText` allowed,
injected `<style>` blocked, `adoptedStyleSheets` allowed), then with a real
Milkdown bundle under the same header — editor created, floating chrome
positioned, tables rendered, zero violations.

An e2e test now asserts the absence of violations in a real browser under the
real path-scoped header, so the claim is pinned rather than argued.

### Bare IDs, no titles

The SPA renders an entity reference as its title, resolved from the server's
per-principal `mentions` map. The app bridge has no such endpoint, and deriving
a title any other way would route around the read gate (BUG-R9EHKV): an entity
the principal may not read must produce no mention at all.

So references render as the bare ID here. That is the same degraded state the
SPA falls back to for an unresolved reference, not a new one, and it keeps the
editor from becoming a second read path into the graph.

### Acceptance Criteria

1. An app needs no change: the six-item contract behaves identically.
Test: `relaEditor.test.ts` drives the real editor (no mock) through the toolbar
and asserts value round-trip, event semantics, readonly, teardown.
2. The two editors cannot serialize a body differently.
Test: the command catalogue, probes, `entityRef` node, serializer contract and
write-back guard are IMPORTED from `components/forms/milkdown/`, not copied;
`editorIcons.test.ts` pins that every command has a glyph.
3. The editor works under the real app CSP in a real browser.
Test: `apps.spec.ts` mounts it in the sandboxed iframe and asserts the served
stylesheet applied, a command runs end to end, and no CSP violation is raised.
4. No webfont ships. Test: an e2e 404 on `_rela-editor.woff2`, a Go test on the
same path, and a unit assertion that every toolbar button drew inline SVG.
5. `.value` is churn-free: a body the app only displayed comes back byte for
byte. Test: `relaEditor.test.ts` round-trips a table and a list.
