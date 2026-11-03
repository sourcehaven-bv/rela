---
id: RR-7UZO26
type: review-response
title: filechannels Rekey is not atomic as documented
finding: Directory move, audit write and old-audit remove are separate steps; a crash splits channels and audit trail. Fix the doc or make the audit move recoverable.
severity: minor
resolution: 'Store doc corrected: fs moves channels in one step and the audit trail in a second; Audit(oldID) still reads a stranded trail.'
status: addressed
---
