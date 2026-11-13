---
id: RR-J2CDV36
type: review-response
title: 'Design: outside_edits exemption covers the whole run'
finding: WithinRun exempts the whole run, so a sibling branch's outside_edits rules do not narrow a branch's step write.
severity: minor
resolution: 'Decision recorded in the plan: intended. The step writes only its own state''s writes: properties, which the flow author listed; outside_edits governs writes from outside the run.'
status: addressed
---
