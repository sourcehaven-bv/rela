---
id: RR-HFDMB0
type: review-response
title: OpenChannel.vue pushed over the 500-line god-component line
finding: 'frontend/src/components/entity/channels/OpenChannel.vue has 503 counted lines after this change (+6 net; develop was under 500). frontend/CLAUDE.md:138 keeps max-lines: 500 to catch god components, and the notes accept the warning knowingly. A warning nobody acts on becomes the baseline for the next ticket. Fix: move the edit state (stopEditing, onEdit, onCancelEdit, editProps at :269-383) into a useComposerEdit composable, or move the decisionsApi dependency literal (:387-410) into a factory in useSessionDecisions, and bring the file back under 500.'
severity: minor
resolution: Moved the paging handlers (onLoadOlder, onLoadNewest, onJump, loadingOlder, JUMP_MAX_PAGES) out of OpenChannel.vue into a new composable, useChannelPaging.ts. eslint gives no max-lines warning on OpenChannel.vue any more.
status: addressed
---
