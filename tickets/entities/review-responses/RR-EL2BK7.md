---
id: RR-EL2BK7
type: review-response
title: Rename affordance is per face while rename is family-wide
finding: '[security] internal/dataentry/affordances.go advertises rename per face, so _actions.rename can be true on the draft face while the family rename is refused. No HTTP rename exists, so this is UI accuracy only.'
severity: nit
reason: 'Out of scope: no HTTP or SPA rename exists. If one is added it should compute the verb across the family with authorizeFamily, as the delete verb would need too.'
status: deferred
---
