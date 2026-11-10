---
id: RR-7IKDW48
type: review-response
title: 'Design: status changes must be compare-and-set'
finding: A plain UPDATE lets Resume turn erasing back into active over a half-dropped schema.
severity: significant
resolution: 'Every status change is a compare-and-set UPDATE on the expected statuses; resume only from suspended, suspend not from erasing. Test: TestProvisioner_Lifecycle.'
status: addressed
---
