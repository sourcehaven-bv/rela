---
id: RR-L9RC9
type: review-response
title: Table cell helper call needs entity.type but cell only has entityId
finding: 'table cells today: navigateToEntity(cell.entityId || row.entityId). The id alone doesn''t tell us the entity type. Either the API needs to expose type per cell (backend change), or fall back to row.entityType for the row-id case (already on row), or skip helper for cells (use cell.link verbatim, fall back to /entity/:type/:id with row.entityType for the id case). Plan must specify.'
severity: significant
resolution: 'Click handler builds entity from { id: cell.entityId || row.entityId, type: row.entityType }. row.entityType is already populated server-side (sections.go:217) -- no backend change required. Pre-existing relation-cell-points-at-row issue accepted as out of scope, code comment flags it.'
status: addressed
---
