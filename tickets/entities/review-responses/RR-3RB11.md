---
id: RR-3RB11
type: review-response
title: EasyMDE toolbar not responsive by default
finding: EasyMDE does NOT have built-in responsive handling. Toolbar buttons overflow on 320px. Needs global (unscoped) CSS targeting .EasyMDEContainer .editor-toolbar with overflow-x:auto. Cannot be hand-waved as a risk.
severity: significant
resolution: Addressed in updated plan PLAN-L6U02
status: addressed
---
