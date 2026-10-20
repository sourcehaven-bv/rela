---
id: RR-JTXH2U
type: review-response
title: reset() and editing-end ordering is a trap for the app
finding: 'RlChannelComposer.vue:141-159, 216-221. reset() clears the current type''s draft synchronously, and the editing watcher restores the stash on the next flush. If the app clears `editing` after a save and calls reset() after an await (nextTick, or the POST answer arriving later), reset wipes the just-restored new-message draft. Fix: make reset() a no-op for drafts while `editing` is set, or have it take the event it acknowledges (reset(seq)). At minimum, document the required order in the defineExpose comment.'
severity: minor
resolution: reset now takes the event it acknowledges, ignores a save event and clears the posted type even when it is stashed behind an edit, with the contract in its doc comment.
status: addressed
---
