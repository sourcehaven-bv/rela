---
id: RR-YBVNAP
type: review-response
title: Group heading drops label_plural without label
finding: The Label guard skipped GetLabelPlural when a type declared only label_plural.
severity: nit
resolution: The guard accepts Label or LabelPlural. Group order stays by type name, as documented.
status: addressed
---
