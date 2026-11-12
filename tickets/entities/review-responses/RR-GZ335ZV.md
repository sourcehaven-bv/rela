---
id: RR-GZ335ZV
type: review-response
title: Error classification misreads deadline and shutdown
finding: A wrapped deadline may not match errors.Is; a queue shutdown cancel would count as retryable.
severity: significant
resolution: 'Plan: classify from the attempt ctx first: deadline means timeout; a non-deadline cancel records nothing and the lost path decides. SMTP classification moves with the mail kind to TKT-VVDY2RM.'
status: addressed
---
