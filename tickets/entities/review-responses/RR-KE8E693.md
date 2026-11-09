---
id: RR-KE8E693
type: review-response
title: Label-row CSS copied into three components
finding: .field-label-row was defined in FieldShell, RelationPicker and RelationCards.
severity: minor
resolution: FieldLabelRow owns the row, button and panel CSS; the three copies are gone.
status: addressed
---
