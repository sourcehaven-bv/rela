---
id: RR-M4PC52
type: review-response
title: Composer overwrites an in-progress edit when the editing prop changes identity
finding: 'RlChannelComposer.vue:141-159. The watcher fires on any new `editing` object, not on a new seq. The app will almost certainly build `editing` as a computed over the held message, so a live frame touching that message (an action status, a rev bump, a reply_count change) creates a new object. The watcher then reseeds drafts[e.type] from the server values and wipes what the user has typed so far. It also cycles the stash: it restores the new-message draft and stashes it again. Fix: watch `() => props.editing?.seq` (plus type), and reseed only when the seq changes. If the message changes under the editor, let the app decide (show a banner) rather than silently replacing the draft. Add a story that swaps in an equal-seq editing object after typing and asserts the typed text survives.'
severity: significant
resolution: The editing watcher now keys on seq and type, so a rebuilt editing object for the same message keeps the typed draft (story EditStashesDraft).
status: addressed
---
