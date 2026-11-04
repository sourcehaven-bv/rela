---
id: RR-890AEID
type: review-response
title: Picker flickers on every session move
finding: useChannel.follow calls setChannel before the head refetch (useChannel.ts:350-372); in that gap decisionViews returns nothing, so buttonTypes empties and the picker and Reply show every type, then shrink.
severity: significant
resolution: 'Plan 2: buttonTypes keeps its previous value while the head generation differs from actionsGeneration; test with the head refetch held open (AC 5).'
status: addressed
---
