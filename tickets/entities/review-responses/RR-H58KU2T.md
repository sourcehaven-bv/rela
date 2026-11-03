---
id: RR-H58KU2T
type: review-response
title: Dense cell logic copied instead of shared with EntityList
finding: denseCell.ts mirrored EntityList columnWidgets/resolveCell, so a future routing change would reach one surface only.
severity: significant
resolution: denseEntry and denseCell in utils/denseCell.ts; EntityList columnWidgets and resolveCell use them; list cell tests pass.
status: addressed
---
