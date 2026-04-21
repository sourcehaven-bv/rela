---
id: RR-OJEIH
type: review-response
title: TestSchemaGraphvizLegendConnectedTargets asserts a broken render
finding: 'The test builds `fans` (4 targets) and `anchors` (4 targets, same targets). Both collapse to legend under the current classifier, so every entity participates only in legend-pairs and `visibleEntities` hides them all. The graph body ends up empty. The test passes because it only checks that `__legend [` appears and `__hub_` does not — neither assertion notices the empty body. This is an instance of critical #2 masquerading as a passing test. Fix: add a non-collapsing edge to keep the body populated and assert on entity labels appearing in node lines.'
severity: significant
resolution: 'Rewrote TestSchemaGraphvizLegendConnectedTargets: two small anchor relations (a1r: 2 targets, a2r: 2 targets) keep every t-node otherwise-connected without themselves collapsing. Assertions now require each of a1/a2/t1..t4 to appear as rendered nodes in the body (catches the empty-body regression) and explicitly verify that src IS hidden (AC 6) since its only pair is legend-collapsed.'
status: addressed
---
