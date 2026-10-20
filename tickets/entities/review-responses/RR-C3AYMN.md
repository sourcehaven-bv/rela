---
id: RR-C3AYMN
type: review-response
title: Scroll anchoring ignores cap trimming, hidden mount and in-place growth
finding: 'RlChannel.vue:113-138. (1) When the app trims the oldest messages at the 500 cap (R19) while a reader is mid-log, content above the viewport disappears and the reader jumps. That case is neither prepend nor append-at-bottom, so nothing compensates. (2) onMounted scrollToBottom does nothing while the panel is hidden (D7 tabs with v-show, or a collapsed section), so the log opens at the top. (3) An edit or an action that makes a message taller does not keep an at-bottom reader at the bottom. (4) The watcher only sees array identity, so in-place mutation (push) never runs it. Fix: anchor on the first visible message''s offset rather than on scrollHeight deltas, or use a ResizeObserver on the list while at bottom. Document that `messages` must be replaced, not mutated. Add play tests for follow-at-bottom and preserve-on-prepend, which are currently untested.'
severity: minor
resolution: Scrolling now anchors on the first visible message for prepends, trims and edits, and a ResizeObserver follows the bottom on growth and on first show; the docs state that messages must be replaced, and play tests cover these cases.
status: addressed
---
