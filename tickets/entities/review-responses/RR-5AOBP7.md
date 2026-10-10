---
id: RR-5AOBP7
type: review-response
title: Undo does not restore item order
finding: Re-adding after Undo appends with new seq values, so order is not restored.
severity: minor
reason: 'Accepted for v1: order is ''order added'' and the pile is a working set, not a curated sequence. A restore token would mean _remove returns data, which reopens the oracle in the first finding. Documented in data-entry.md.'
status: wont-fix
---
