---
id: RR-AF6SJ
type: review-response
title: handleToggleCheckbox missing writeMu
finding: /api/toggle-checkbox handler calls a.ws.UpdateEntity (a mutation) without taking a.writeMu. The App struct doc says every mutation path must take writeMu; this one doesn't, so it can race with any other mutation and corrupt the live graph node mid-update.
severity: critical
resolution: handleToggleCheckbox now takes a.writeMu before reading the entity and calling UpdateEntity. The handler also clones the live entity twice (once for oldEntity, once for the working copy) so it doesn't mutate the live graph node in place — addressing the related S5/RR-1CO05 issue inline for this specific handler.
status: addressed
---
