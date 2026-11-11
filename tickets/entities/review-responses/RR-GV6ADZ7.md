---
id: RR-GV6ADZ7
type: review-response
title: Question components focus buttons by another component's class
finding: RlChannelQuestions and RlChannelMessage find the choice buttons by a scoped class name.
severity: nit
reason: Both selectors live in the same library and a rename shows in the Storybook focus tests; worth a focusChoices method when the message component is next touched.
status: deferred
---
