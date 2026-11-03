---
id: RR-I3EV4O
type: review-response
title: Range width check can overflow
finding: Computing max-min+1 as int for the 50-value cap and for the observed fill overflows for extreme bounds such as min -9223372036854775808.
severity: minor
resolution: 'Compare widths without overflow: check hi-lo against the cap using uint64 of the difference after ordering, and stop filling at lo+49.'
status: addressed
---
