---
id: RR-4R3XQP
type: review-response
title: EasyMDE preview kept the original squeezing bug; the mirror test cannot detect it
finding: 'The .editor-preview alias block had no min-width/max-width/overflow-wrap while .editor-preview itself sets overflow-wrap: anywhere, so the live preview still rendered the squeezed columns the ticket exists to fix. markdownContentMirror.test.ts guards SPA-vs-app-editor drift, not .md-body-vs-.editor-preview drift, so it passes regardless.'
severity: significant
resolution: Mirrored the three cell declarations into both .editor-preview blocks (markdown-content.css and relaEditorTheme.css) with a comment stating the preview intentionally has no scroll wrapper, since EasyMDE builds its preview through its own marked instance. Mirror test still passes, confirming the copies stayed in sync.
status: addressed
---

# Finding

The `.editor-preview` alias block carried no `min-width`, `max-width` or
`overflow-wrap: break-word`, while `.editor-preview` itself sets `overflow-wrap:
anywhere` — the exact precondition of the bug. So the EasyMDE live preview still
rendered "Nive au" at 58px while the saved entity rendered correctly,
contradicting the stylesheet's own header comment that the preview exists to
"show what the entity view will actually render".

`markdownContentMirror.test.ts` passes regardless: it compares `.editor-preview`
rules **between the two files**, so it guards SPA-vs-app-editor drift, not
`.md-body`-vs-`.editor-preview` drift. That second axis has no guard, which is
why the gap was silent.

# Resolution

Already fixed before this review landed — the gap was found while re-checking
the preview surface, and the reviewer was reading a slightly older tree.

The three cell declarations are mirrored into **both** `.editor-preview` blocks
(`markdown-content.css` and `app-editor/relaEditorTheme.css`), each with a
comment recording that the preview deliberately gets no `.md-table-scroll`
wrapper: EasyMDE builds its preview HTML through its own marked instance, which
never sees rela's renderer. So in the preview the squeezing is fixed and the
scroll affordance is absent — a documented difference rather than a silent one.

`markdownContentMirror.test.ts` passes, confirming the two copies stayed
identical.

**Not done:** adding a guard for the `.md-body`-vs-`.editor-preview` axis. It
would have caught this, but the two are not meant to be identical (the preview
has no wrapper by design), so a naive equality test would be wrong. Worth a
follow-up that compares only the declarations the two surfaces genuinely share.
