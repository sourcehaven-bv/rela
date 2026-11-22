---
id: RR-Z68ZDB9
type: review-response
title: "keepInView duplicates the router's anchor settle loop"
finding: "scrollToAnchorWhenReady in router/index.ts kept its own settle loop with a MutationObserver, render events and scroll listeners next to the new keepInView."
severity: significant
resolution: "Addressed: the router finds the anchor and hands it to keepInView, which gained an abort option for a navigation away. One loop now serves both, and its listeners and idle stop are pinned in keepInView.test.ts."
status: addressed
---
