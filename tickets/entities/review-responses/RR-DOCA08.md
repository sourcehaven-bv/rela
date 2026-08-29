---
id: RR-DOCA08
type: review-response
title: 'a wrong status masked a wrong error code'
finding: 'checkAPI returned on the first failing claim, so api{status=..., error=...} with both wrong reported only the status — costing an extra fix-and-rerun cycle on a red build.'
severity: minor
resolution: 'Both claims are evaluated and reported together.'
status: addressed
---
