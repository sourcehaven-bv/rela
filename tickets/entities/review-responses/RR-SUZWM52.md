---
id: RR-SUZWM52
type: review-response
title: A side with an id but no type was skipped
finding: The detail served that side's values for an id the gate never judged.
severity: minor
resolution: A side that has values but no complete identity makes the file unjudgeable, so it is hidden. An empty side (deleted on one branch) is still allowed.
status: addressed
---
