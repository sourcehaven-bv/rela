---
id: RR-YTC4W5
type: review-response
title: 'Plan finding 1 is false: ViewCell.Widget is live, not dead'
finding: 'PLAN-DMQFRJ Research §1 and TKT-3R7RF3 both assert ViewCell.Widget is ''populated nowhere'' and ''dead on the wire'', citing `grep ''Widget:'' internal/dataentry` returning no hits. The grep pattern was structurally incapable of finding the assignment, which is a field assignment not a struct-literal key: `cell.Widget = resolveWidget(pd, s.Meta)` at internal/dataentry/sections.go:296. It reaches the wire via the unnamed conversion `v1.ViewCell(cell)` at views_handler.go:611 and :629, so every table cell of every view ships a widget name today. Consequences: (a) the Non-goals in both ticket and plan are justified by a false premise, risking a future reader deleting a live wire field; (b) it means a SECOND server-side type->widget resolver already exists (resolveWidget -> Metamodel.ResolveWidgetFromType, schema_output.go:117) whose godoc claims to be ''the single source of truth'' -- a claim already false, since registry.ts defaultWidgetFor is the authority on the section path.'
severity: critical
resolution: 'Verified independently: `grep -rn Widget internal/dataentry/sections.go` shows the assignment at :296. Finding confirmed as my error. Corrected Research §1 in PLAN-DMQFRJ and the Scope-corrections section of TKT-3R7RF3 to state ViewCell.Widget is live and type-derived; re-justified the scope exclusion on surface grounds (tables are a distinct renderer with no inline-edit path) rather than on dead-code grounds.'
status: addressed
---
