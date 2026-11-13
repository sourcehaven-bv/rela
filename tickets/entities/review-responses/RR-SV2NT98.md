---
id: RR-SV2NT98
type: review-response
title: Question grouping splits when the first question moves the session
finding: groupQuestions ended a run at the session notice that lands after a moving first question; the reader saw Q1 alone and then Question 1 of 2.
severity: significant
resolution: A notice row or a fold of notices only no longer ends a batch run; it is drawn right after the group. Unit tests cover the Q / notice / Q / Q order.
status: addressed
---
