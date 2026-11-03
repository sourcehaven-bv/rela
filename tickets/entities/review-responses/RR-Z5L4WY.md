---
id: RR-Z5L4WY
type: review-response
title: Edits are deliberately exempt from in:, but the docs never say so
finding: 'The plan (Out of scope, Edge cases) decides that editing a stored message whose type is now outside its states is allowed, because edits never move a session. msgops.go:175 confirms that editGate checks mayPost and requireOpen, not in:. The new "Where a message type may be posted" bullets (docs/realtime.md:588-604, GUIDE-realtime.md mirror) cover posts, triggers, typing, stranded and final states, but not edits. An operator who reads "in: limits the type to the states it lists" will expect an approve to be uneditable in working, and an SPA user will see Edit on a message whose type the composer no longer offers. Fix: add one bullet, "in: governs new posts and replies only; an edit of a stored message is judged by edit_own as before."'
severity: minor
resolution: 'Added a docs bullet: in: governs new posts and replies only, and an edit is judged by edit_own as before.'
status: addressed
---
