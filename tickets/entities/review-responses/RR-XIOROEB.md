---
id: RR-XIOROEB
type: review-response
title: "Dragging the scrollbar may not stop the loop"
finding: "Only wheel, touchmove, keydown and pointerdown count as the user taking over."
severity: minor
reason: "A scroll event cannot tell the user from a layout shift or the browser's scroll anchoring, and treating it as the user would stop the loop in the case it exists for. Pointerdown covers scrollbar presses in Chromium and WebKit, and the loop ends on its own after the page settles."
status: wont-fix
---
