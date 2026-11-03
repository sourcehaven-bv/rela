---
id: RR-PQE2MR
type: review-response
title: The announcement filter drops kept rows a person must act on
finding: 'Plan step 6 and AC 8 drop added messages of a quiet type from the polite announcement (RlChannel.vue:234-259). A hidden-type question with choices, or a quiet message that is the decision cause, still renders in full as a kept row, but it is not read out. This is the one message a screen-reader user must hear about. Change the plan: filter on the fold result (drop only messages that do not render as a full row), using the same keep set. Also handle an empty list after filtering, because line 258 dereferences messages! when there are no notices.'
severity: significant
resolution: The announcement now keeps only added messages that render as a full row, using the fold result and the same keep set, and returns early when nothing is left so messages! is never read empty (step 6, AC 8). Added the cases "kept quiet row is announced" and "only quiet additions announce nothing".
status: addressed
---
