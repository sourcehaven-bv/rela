---
id: RR-5WQSAT
type: review-response
title: Section table is inline in EntityDetail and grouped sections re-sort
finding: Not RlTable; buildSections re-sorts groups by id (sections.go:415).
severity: significant
resolution: 'Plan updated: Reorder logic lives in a composable used by RlTable and the inline section table; sections with group_by are not orderable.'
status: addressed
---
