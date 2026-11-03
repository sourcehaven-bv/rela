---
id: RR-38TDPI
type: review-response
title: Remove erases a pending action and its pending_at so a stuck promote becomes invisible
finding: sqlchannels Remove rebuilds the message without Action and update() then writes pending_at=0. A moderator removing a message during or after a promote crash makes SetAction(done) fail with ErrRemoved, hides it from StalePending, and leaves the entity with no channel record. Refuse Remove and Edit while an action is pending (ErrConflict). Build promote props from the message returned by the pending SetAction CAS rather than the step-2 Get, so a concurrent edit cannot change what gets promoted.
severity: significant
resolution: Remove and Edit refuse a message whose action is pending (409 action_pending). Promote props are built from the message returned by the pending CAS.
status: addressed
---
