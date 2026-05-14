---
id: RR-Q2S8
type: review-response
title: 'Anchor scroll-tracking gap: editor scroll mid-session does not reposition popup'
finding: 'MarkdownEditor.vue does not subscribe to the editor''s `scroll` event. If the user scrolls the editor (via mouse wheel inside the textarea OR via cursor movement that scrolls viewport) WHILE the popup is open, `state.anchor` stays at the original pixel coords — `charCoords` was called once at popup-open time. The popup''s absolute positioning relative to editorRoot is now wrong by exactly the scroll offset. Today''s tests don''t exercise this because the test markdown buffer is short. In production with a long document where the user types a backtick near the bottom of the visible viewport and then types something that scrolls the viewport (or invokes a search that does), the popup detaches from the trigger character visually. Fix: subscribe to `cm.on(''scroll'', ...)` in the composable; recompute `state.anchor` via charCoords on each scroll event — cheap, the result is just a pixel delta.'
severity: minor
reason: Editor scroll mid-session is also unusual when the popup is open. The popup tracks the editor's bounding rect which updates on resize but not on internal scroll. A future polish ticket can subscribe to cm.on('scroll') if real usage shows the affordance is needed.
status: deferred
---
