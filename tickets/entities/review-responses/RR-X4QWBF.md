---
id: RR-X4QWBF
type: review-response
title: Committed value must be bare _title, not 'Title (ID)'
finding: 'RelationPicker''s formatEntityLabel (RelationPicker.vue:248) uses entityDisplayTitleWithId, which returns ''Title (ID)'' when title!=id (entityDisplay.ts:32). The backend matches on bare DisplayTitle (api_v1.go:476: DisplayTitle(...) == want). If the extracted EntityTargetSelect reflexively copies RelationPicker''s label logic, the committed filter value becomes ''Jeroen Vloothuis (PERS-001)'' which never satisfies the equality match — filter silently returns empty.'
severity: critical
resolution: 'Plan pins: committed value = entityDisplayTitle(entity) (bare _title, entityDisplay.ts:25), NEVER entityDisplayTitleWithId. Display label in the dropdown MAY show ''Title (ID)'' for disambiguation, but option VALUE written into localFilters[relation] must be the bare title. Component design separates ''option display label'' from ''option value''.'
status: addressed
---
