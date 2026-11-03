---
id: RR-9QUIKY
type: review-response
title: Faced e2e does not prove channels are per face
finding: 'channels.spec.ts:172-185 opens the same face for both users, so it does not show that two faces have separate channels. Add a step where one user opens the other face (draft) and asserts that the published-face message is absent and that a post there does not reach the published-face reader. That separation is what `anchor: face` exists for.'
severity: minor
resolution: The faced e2e now opens the draft face as a second context, posts there, and asserts the draft post reaches neither published-face reader and the published posts do not appear on the draft face.
status: addressed
---
