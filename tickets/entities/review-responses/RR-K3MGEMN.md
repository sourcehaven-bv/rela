---
id: RR-K3MGEMN
type: review-response
title: Duplicate successor ids in ResolveMentions
finding: Several retired ids collapsing into one successor put that id in the lookup more than once.
severity: nit
resolution: ResolveMentions sorts and compacts the successor targets before resolving.
status: addressed
---
