---
id: RR-MI63AC
type: review-response
title: Confirm path posts against the generation read after the dialog, not the one the user decided on
finding: 'frontend/src/components/entity/channels/useSessionDecisions.ts:149-160 reads `generation` from the head only after `await d.confirm(...)` and the re-`find(key)`. The re-find matches by key, and a key such as `abandon` or `accept` can exist again at a newer generation: in a re-entered state (review -> working -> review) or in another state that offers the same type. If the session moves while the dialog is open and the fresh head lands before the user clicks Confirm, the post carries the new generation. The server then accepts a decision against a state the user never saw. This defeats the reason expect_generation exists (plan Problem; AC 10). The test ''the session moved while the dialog was open'' (ChannelsPanel.test.ts) only moves to a state with no decisions, so it does not catch this. Fix: capture `const generation = head.channel.session.generation` synchronously at the top of onDecide, before any await, and send that. Ideally carry it from the rendered view or the emitted event. Then the server answers session_moved, and the re-find becomes a UX nicety rather than the guard. Add a test: open the dialog, re-enter the same state at generation+2 with the same transitions, confirm, and expect a POST with expect_generation 4 (or no POST).'
severity: significant
resolution: onDecide now reads the generation from the head before its first await and sends that value, so a re-entered state answers session_moved; pinned by the panel test "the same state came back at a newer generation while the dialog was open" and the unit test "posts with the generation read at the click".
status: addressed
---
