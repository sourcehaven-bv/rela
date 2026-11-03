---
id: RR-AYJVWI
type: review-response
title: useMessageLink drops a change that lands during its one-tick wait
finding: 'frontend/src/components/entity/channels/useMessageLink.ts:22-36: while the first call awaits nextTick, `waiting` makes every other watcher call return at once. The first call then sees seq() !== s or view.value !== v and also returns. Nobody calls jumpTo or onDone for the new value. pendingReveal stays set and seq stays in the URL until some other dependency changes. Fix: drop the `waiting` flag and re-read the current values after the tick (if seq() is still defined and view.value is set, jump to the current seq), or use watch with flush: ''post'' instead of the manual nextTick.'
severity: minor
resolution: 'ADDRESSED dropped the waiting flag: each watcher call waits one tick and stops if seq or the view changed, since the change runs the watcher again and that call reveals the new seq. New useMessageLink.test.ts pins this, including a change inside the same flush (fails on the old code).'
status: addressed
---
