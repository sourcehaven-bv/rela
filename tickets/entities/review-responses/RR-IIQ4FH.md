---
id: RR-IIQ4FH
type: review-response
title: Post-update renumber of _order_in rewrites other sources' edges
finding: runRenumberAfterUpdate renumbers the incoming side after a direct meta write of _order_in. It rewrites every edge into the target without checking that the caller may update them. This predates the ticket and runs only on a collapsed gap.
severity: minor
reason: Predates this ticket and runs only after a direct meta write of _order_in leaves a collapsed gap, not on a move. Changing it needs its own authorization design for system renumbers; out of scope here.
status: deferred
---
