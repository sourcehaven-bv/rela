---
id: RR-XPFE05
type: review-response
title: No test for moving from one entity page to another
finding: PageView is reused when the entity prop changes. Nothing pinned that current is cleared and opening set while the second anchor loads, so a later placeholderData on the anchor query could pair entity B's id with entity A's face ref.
severity: significant
resolution: 'Added ''opens again when it moves to another entity'': after TOP-1 loads, setProps to TOP-2 with a pending fetch gives opening true and current undefined; after it resolves, current names TOP-2 with its own ref.'
status: addressed
---
