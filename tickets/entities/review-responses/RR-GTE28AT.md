---
id: RR-GTE28AT
type: review-response
title: Padding carries no relations
finding: Padded fixtures add entities without edges, so a relation scan returns the same rows at both sizes.
severity: significant
resolution: The shared budget harness, owner, gantt, write-reads, command-input, roll-up and inbox padding now carry edges of the relation types the operation reads, to targets only the padding uses.
status: addressed
---
