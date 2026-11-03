---
id: RR-ZV33E1
type: review-response
title: Line budget estimate and template structure
finding: 'RlChannel.vue counts about 408 lines under the max-lines options today (about 92 to spare, not 70), and OpenChannel.vue about 481, so leaving OpenChannel untouched is right. But an expanded run needs its messages inside an id''d container for aria-controls. With a plain v-for, the 21-line RlChannelMessage binding (RlChannel.vue:373-393) then appears twice, and together with the toggle, rows, jumpTo, announce, signal filter, empty text and CSS that exceeds the margin. Change the plan: move ephemeralText to channelFields.ts up front, not as a fallback, and choose the row structure now (one v-for over a flattened row list, or a small row component). max-lines is warn-only (eslint.config.js:152, CI runs eslint without --max-warnings), so this is hygiene, not a gate.'
severity: minor
resolution: 'Corrected the budget to about 92 spare lines, moved ephemeralText to channelFields.ts up front, and settled the row structure: one v-for over rows with a nested ol for an expanded fold and the RlChannelMessage binding written once via messageBind and messageOn (step 6). The plan notes that max-lines is warn-only.'
status: addressed
---
