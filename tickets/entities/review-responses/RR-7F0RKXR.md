---
id: RR-7F0RKXR
type: review-response
title: Arrived branches apply the join state's outside_edits
finding: statesOf includes arrived branches, whose state is the join, so the join's rules apply before it fires.
severity: minor
reason: 'Deliberate: a branch at the join is in the join state, and its rules describe that state. The guide says the rules of every state the run is in apply.'
status: wont-fix
---
