---
id: RR-KEN3QD5
type: review-response
title: Re-entry erases the previous state
finding: Storing Previous = move.From on a re-entry (B to B) and hiding a previous equal to current loses the real previous state for good on repeated self-transitions.
severity: significant
resolution: ApplySession keeps c.Session.Previous when move.From equals move.To. AC 4 and a conformance case.
status: addressed
---
