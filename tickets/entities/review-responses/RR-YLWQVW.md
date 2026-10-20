---
id: RR-YLWQVW
type: review-response
title: An edit whose message left the held list erases hidden props
finding: 'onSave (OpenChannel.vue:235-256) looks up `held` in ch.messages at save time and passes `held?.props` to toEditProps. When the message is no longer held, `current` is undefined, so every unrendered and hidden prop is left out and the PATCH, which replaces the props (R7), silently erases them. Only restricted refs survive, because the server keeps unnamed refs. The message leaves the list during an edit whenever useChannel reloads: a frame needing more than MAX_FOLLOW_PAGES pages (a busy channel), loadNewest, or the 500-message cap after loadOlder. This is the read-modify-write erasure the repo CLAUDE.md warns about. Fix: snapshot the stored props when the edit starts (on the ChannelEditing value from editingFor, or in a local ref set in onEdit) and build the PATCH from that snapshot; if no stored copy exists, refuse the save with a form error rather than sending a partial body. Add a ChannelsPanel test that starts an edit, forces a reload that drops the message, saves, and asserts the hidden prop is in the PATCH.'
severity: significant
resolution: OpenChannel snapshots the message props when the edit starts and builds the PATCH from that snapshot, refusing the save with a form error when no snapshot exists, and a ChannelsPanel test drops the message through a capped follow reload before saving and asserts the hidden prop is sent.
status: addressed
---
