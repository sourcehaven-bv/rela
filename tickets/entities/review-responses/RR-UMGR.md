---
id: RR-UMGR
type: review-response
title: FontAwesome icon dependency is undocumented — picker breaks silently if FA is unbundled
finding: |-
    `MarkdownEditor.vue:56` sets the custom toolbar button's `className: 'fa fa-at'`. The comment says 'FontAwesome v4 (bundled with EasyMDE) already ships the icon so no asset cost.' That's accurate today, but creates an invisible coupling:

      - EasyMDE bundles FontAwesome 4.7's icon font CSS in `easymde.min.css` (which the editor imports at the top of the file). Future EasyMDE versions might switch to inline SVGs or drop FA in favor of a smaller icon font — the codebase has no signal that the picker depends on the `fa fa-at` class.
      - If a future contributor splits CSS bundles or tree-shakes EasyMDE's CSS, the button renders as a blank rectangle and the user has no idea what it does (the `title` attribute is the only fallback).

    Low-effort fixes:
      - Add a fallback text/SVG. EasyMDE's button styling supports both `className` and `text`: setting `text: '@'` as a sibling would render '@' if the icon font fails to load.
      - Better: ship our own inline SVG via `innerHTML` on the toolbar button after EasyMDE mount. Removes the FA dependency entirely.
      - At minimum: add an e2e or visual-regression assertion that the button shows the '@' glyph, so an icon-font-loss regression fails CI instead of silently degrading the UX.

    The e2e test currently locates the button by `title="Insert entity reference"`, so it would pass even if the icon never rendered. That's the wrong selector for catching this class of bug.
severity: nit
reason: EasyMDE bundles FontAwesome and the existing buttons (link, code, quote) already depend on it -- adopting fa-at extends the same coupling, not a new one. The e2e selects by title=, not by class, so an icon-render regression in a future EasyMDE version would be caught by visual inspection rather than the e2e -- accepted trade-off.
status: deferred
---
