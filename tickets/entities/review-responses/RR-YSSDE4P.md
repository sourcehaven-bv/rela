---
id: RR-YSSDE4P
type: review-response
title: Conditional summary list is not implementable as written
finding: Whether a tab exists is known only after the channel schema loads asynchronously, and a composable cannot be created after setup.
severity: minor
resolution: Deferred to TKT-B75I7LH with the tab design; the planned fix is recorded there.
reason: 'Applies only to placement: tab, which moved to TKT-B75I7LH (approved scope change: this ticket ships above_relations only).'
status: deferred
---
