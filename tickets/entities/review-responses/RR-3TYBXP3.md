---
id: RR-3TYBXP3
type: review-response
title: 'Design: completion work depends on whether a hidden field is set'
finding: Completing only pending rows makes the response time differ between 0 and 1 pending rows in a range; a peer can learn whether a hidden property is set on one row.
severity: significant
resolution: Every row of a type whose fold is not FoldStored is loaded and redacted and hashed in Go whether pending or not; stored hashes are used only for FoldStored types. The budget test compares 0 and N hidden values.
status: addressed
---
