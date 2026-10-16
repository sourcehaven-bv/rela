---
id: RR-OUVQDC
type: review-response
title: Face-delete last-face recheck skipped when nothing detached
finding: After the no-channels optimisation, faceDeleteGuard.active is len(undo) > 0, so errFacesChanged is checked only when something was detached. A concurrent sibling face delete can make the caller's face the last one inside the Tx and a block-mode channel no longer stops the delete.
severity: significant
status: open
---
