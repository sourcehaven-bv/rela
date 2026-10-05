---
id: RR-I2JTI5
type: review-response
title: Zero-coordinate rationale duplicated at five call sites
finding: The same comment explaining the empty incoming face was repeated at each site.
severity: nit
resolution: Hoisted into incomingOwnedAtZero, whose doc also states that on these surfaces the filter is defense in depth rather than the gate.
status: addressed
---
