---
id: RR-J1RDM
type: review-response
title: scrollBehavior races user scroll + scroll-to-top on timeout stomps position
finding: 'frontend/src/router/index.ts:96-103 waitForElement polls up to 2s. Two real UX bugs: (1) if the user scrolls manually during the wait, a resolve at ~1.8s smooth-scrolls the page out from under them; (2) on timeout (element never appears, e.g. script error), the handler returns {top:0} and jumps them to the top — they lose their reading position. Fix: snapshot to.fullPath at start; if router.currentRoute.value.fullPath changed during wait, bail with current scrollX/scrollY. On timeout, return current position instead of {top:0}. Consider MutationObserver on .document-body instead of rAF polling.'
severity: significant
resolution: 'scrollBehavior snapshots startPath + startScrollY, passes an abort callback into waitForElement that stops polling if either changes. On timeout/abort, returns {left: scrollX, top: scrollY} instead of {top: 0} so the user''s current position is preserved. If the target element appears legitimately, smooth-scroll happens as before.'
status: addressed
---
