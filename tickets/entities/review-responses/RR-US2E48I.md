---
id: RR-US2E48I
type: review-response
title: Colour errors hidden while an axis is invalid
finding: validateKanbanCellColors returned early when the column or lane property was invalid, so bad colours surfaced only on the next load.
severity: minor
resolution: Only the key checks on the invalid axis are skipped; every colour and label is still checked. Pinned by TestValidateConfig_KanbanCellColorsCheckedWithABadAxis.
status: addressed
---
