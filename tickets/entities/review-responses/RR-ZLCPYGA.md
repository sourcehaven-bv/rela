---
id: RR-ZLCPYGA
type: review-response
title: 'Design: header patches from sibling branches race'
finding: Header status computed per branch and patched outside the CAS can land out of order and drop a waiting run from Tasks.
severity: significant
resolution: 'Plan updated: the header is written once at the fork (state = join, status input_required when any region state waits for a role, else working) and again only when the join fires or the run ends.'
status: addressed
---
