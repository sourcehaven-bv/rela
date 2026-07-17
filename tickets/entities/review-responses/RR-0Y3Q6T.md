---
id: RR-0Y3Q6T
type: review-response
title: Safety-cap truncation surfacing is unspecified — risks reintroducing the bug at a higher ceiling
finding: 'The plan says the page-count safety cap should ''surface, not hide, truncation'' but names no mechanism, no cap value, and the kanban board has no truncation UI. The bug being fixed IS silent truncation; a cap that only console.warns recreates it at 10k entities. The plan must specify: the cap value, and a user-visible signal (uiStore toast or board banner) when the cap is hit.'
severity: significant
resolution: 'Plan revised: cap fixed at 50 pages (~5,000 entities). On cap hit the merged response preserves meta.has_more: true (api layer stays store-free per its purity rule) and KanbanView renders a board-level warning banner — ''Showing N of TOTAL items — board truncated''. A complete fetch always returns has_more: false. Both banner states are asserted in the component regression test.'
status: addressed
---
