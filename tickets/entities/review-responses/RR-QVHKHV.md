---
id: RR-QVHKHV
type: review-response
title: Icon slot dropped with pending-label and no icon prop
finding: reservesSpinner checked props.icon but not the icon slot, so a slotted icon was never rendered on a pending-label button.
severity: nit
resolution: reservesSpinner now also checks useSlots().icon.
status: addressed
---
