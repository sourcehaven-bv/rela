---
id: RR-KO06FAC
type: review-response
title: 'Design: undecided join and fork rules'
finding: Join guard errors, order of arrivals, what a join state may declare, re-forking after a loop and several forks from one state were undecided.
severity: minor
resolution: 'Plan updated: a join guard error counts as false and is logged; branches settle in fork target order; a join state is an ordinary state once entered (outputs, on, after, waiting_for), its timers count from entry; a loop back to the fork state forks afresh; several fork branches from one state are allowed when their regions and joins are distinct. Each has a test.'
status: addressed
---
