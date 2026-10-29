---
id: RR-6HHO5U
type: review-response
title: Kept rows disappear while the reader is using them
finding: 'Kept rows depend on state that changes under the reader. After a decision, the head refresh changes decisionSeq (useSessionDecisions.ts:151-155). After an answer, the question''s entry leaves choices (useSessionDecisions.ts:167-187). A quiet row that was kept then becomes hidden or folded. useSessionDecisions.ts:315 and :374 call focusRow on exactly that row right after postView, so focus lands on it and is then lost when the next render removes it. :374 ignores the false result, so there is no fallback to focusHeader either. The reader also loses sight of the question they just answered. Change the plan: once a seq has been kept during this mount, it joins revealed and stays kept. Add a ChannelsPanel case: answer a hidden question with choices, and its row and focus remain.'
severity: significant
resolution: revealed now only grows during a mount, and a watch adds every kept seq to it, so a row stays kept after its decision or answer is gone (step 6, AC 4). Added ChannelsPanel cases for the decided cause and the answered question keeping their row and focus.
status: addressed
---
