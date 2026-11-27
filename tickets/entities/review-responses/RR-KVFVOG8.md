---
id: RR-KVFVOG8
type: review-response
title: "Assertion loop passes with an empty want map"
finding: "TestRunString_OutputFields loops over tc.want, so a future case with no expectations checks nothing."
severity: nit
reason: "No case has an empty map; adding a guard is speculative."
status: wont-fix
---

TestRunString_OutputFields loops over tc.want, so a future case with no expectations checks nothing.
