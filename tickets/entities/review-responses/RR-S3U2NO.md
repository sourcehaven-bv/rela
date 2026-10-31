---
id: RR-S3U2NO
type: review-response
title: Edit end re-applies compose and moves the focus
finding: 'RlChannelComposer.vue:261-271: when an edit ends while compose is set, the key watcher fires again (its source goes from empty to key), reselects the type and calls focusField. A user who just saved an edit then has the focus pulled into the compose form. Consider restoring compose mode without moving the focus when the key did not change.'
severity: nit
resolution: "addressed\tThe composer remembers the last compose key it applied. When an edit ends with the same key, it selects the compose type unless the user left it, and does not move the focus. Pinned by the ComposeModeAfterEdit story."
status: addressed
---
