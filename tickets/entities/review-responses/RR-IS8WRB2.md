---
id: RR-IS8WRB2
type: review-response
title: Focus on an expanded fold lands on a message row after a re-key
finding: "The hand-off tried [data-seq] first, so an expanded fold re-keyed by Load older handed its button focus to the message row inside the expanded list."
severity: minor
resolution: "focusedRow returns { seq, fold }; a fold button's focus goes to the fold button that now holds the seq first. Test for an expanded fold re-keyed by Load older."
status: addressed
---
