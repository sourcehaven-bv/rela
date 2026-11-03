---
id: RR-7VA7O0
type: review-response
title: Fold button label and ARIA give the same signal twice
finding: 'AC 5 changes the label from ''3 more messages: ...'' to ''Hide 3 messages'' and also toggles aria-expanded, so a screen reader says ''Hide 3 messages, expanded''. A disclosure should keep its name and let aria-expanded carry the state, or change its name without aria-expanded. Not both. Also, aria-controls points at a container that exists only while expanded. Either always render the container with hidden, or set aria-controls only when expanded. Also say in the plan that the fold is an li in the messages ol with its run in a nested list, so the log''s list semantics stay valid.'
severity: minor
resolution: The fold text no longer changes with the state, aria-expanded carries it, aria-controls is set only while expanded, and the plan states the fold is an li with its run in a nested list (AC 5, step 5, Q4).
status: addressed
---
