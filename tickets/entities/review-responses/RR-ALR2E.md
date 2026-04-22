---
id: RR-ALR2E
type: review-response
title: Modifier-click + middle-click nuked by preventDefault
finding: 'frontend/src/composables/useDocumentClicks.ts:34 calls event.preventDefault() unconditionally. Users cmd/ctrl-clicking to open in a new tab, middle-clicking, or shift-clicking (new window) get neither the default browser action nor SPA navigation — the link is effectively dead. Pre-existing in DocumentView too; my composable propagated it to both surfaces. Fix: early-return before preventDefault if event.button!==0 OR any of metaKey/ctrlKey/shiftKey/altKey OR anchor has target="_blank" or download attribute. Add test cases.'
severity: significant
resolution: createDocumentClickHandler early-returns when event.button!==0 OR any of metaKey/ctrlKey/shiftKey/altKey is set. Also leaves target="_blank"/_other and download-attribute links to the browser. 8 new test cases (meta/ctrl/shift/alt/middle/right click + target=_blank + download) in useDocumentClicks.test.ts. Default preventDefault path only fires for plain left-click on a same-origin link without target attribute.
status: addressed
---
