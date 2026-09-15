---
id: TKT-YH0HDL
type: ticket
title: Verify the renderer security assumptions behind Milkdown HTML passthrough and flattenToLine
kind: enhancement
priority: medium
effort: s
tags: security
status: done
---

## Description

Two IB-review findings (GitHub #1597, #1594) point at the same gap: a security
property that is genuinely true, argued correctly in prose, and never executed
by a test. Neither is a live vulnerability. Both are assumptions that a
dependency upgrade could silently invalidate, because nothing in this repo
asserts them.

### #1597 — raw-HTML passthrough in the Milkdown editor

CommonMark lets an entity body carry literal HTML, and the `commonmark` preset
keeps it as an `html` node rather than dropping it, so the bytes do reach the
editor. It is safe because that node's `toDOM` returns the ProseMirror spec
`['span', attrs, node.attrs.value]`, whose third element is a text CHILD —
ProseMirror builds it with `createTextNode`, not by assigning `innerHTML`.

TKT-3I9DDY's security note claimed "no HTML is executed by the editor". True,
but the claim rests entirely on a property of a third-party node we do not own.
Milkdown has shipped this exact vulnerability class twice in adjacent nodes
(CVE-2026-57530 link href, CVE-2026-57531 emoji innerHTML sink, both fixed in
7.21.3; we are on 7.22.1). A release that switched this node to a DOM-parsing
renderer would have passed CI unnoticed.

### #1594 — marked/goldmark parity for line-ending whitespace

`flattenToLine` replaces only `\n` and `\r` in an interpolated webhook value,
leaving `\v`, `\f`, U+0085, U+2028 and U+2029 in place — see RR-2HD5RQ on
TKT-02V29V, which established the rune set. As its godoc says, that is safe
because of what the PARSER treats as a line ending, not because of anything the
function does.

So the guarantee belongs to the renderers, and it had been verified against one
of the two: goldmark on the write path has Go tests; marked on the read path had
none. A divergence on any single character would reopen the forged
sibling-heading vector TKT-02V29V closed.

## Change

Add the two missing test suites. No production behaviour changes.

- `frontend/src/components/forms/milkdown/rawHtmlPassthrough.test.ts` mounts the
real editor and asserts the observable consequence — no element created, no
handler, no navigable `javascript:` href — rather than the DOM-spec shape, so
the tests survive a reimplementation of the node. It also pins that passthrough
is lossless in both directions, since a sanitizer added at the parse step
instead of the render step would silently rewrite stored bodies.
- `frontend/src/utils/markdownLineEndings.test.ts` renders the webhook delivery
shape through `renderMarkdown` for each of the five survivors.
- One godoc line on `flattenToLine` pointing the claim at its evidence.

## Acceptance criteria

1. Opening a body containing `<img src=x onerror=alert(1)>` creates no element
and runs no handler; the markup survives as visible text.
2. Raw HTML round-trips back to the stored markdown unchanged.
3. None of `\v`, `\f`, U+0085, U+2028, U+2029 forges a heading in marked, and
the value text is never dropped.
4. Every assertion is checked against the bug it names, so a suite that cannot
fail is not mistaken for evidence.

## Verification

Each negative assertion was run against a simulated regression:

- Patching the preset's `html` node to an `innerHTML` sink fails three of the
Milkdown tests.
- Neutering `sanitizeLinkHref` (pre-7.21.3 behaviour) fails the fourth.
- The marked suite carries `\n` and `\r` positive controls that MUST forge a
heading; if they stop, the other five cases are no longer evidence.

The dependency was restored and the suite re-run green after each.

## Security note

Both suites are negative assertions — they claim something does NOT happen. That
kind of test passes just as convincingly when it has stopped testing anything,
which is why the failure-mode checks above are part of the work rather than a
nicety.
