---
id: RR-76KSA7
type: review-response
title: Grouped table rows drop _self
finding: The grouped-row wire conversion does not copy Self (views_handler.go:1039), so faced rows in a columns_by_type table lose their face for edit navigation.
severity: significant
resolution: viewRowToV1 converts flat and grouped rows alike, so grouped rows carry _self. Asserted in TestViewMentions_TableGroupsByType.
status: addressed
---
