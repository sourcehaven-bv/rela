---
id: RR-EIXM
type: review-response
title: Stale doc comment on resolveSectionButtonsWithTraverse
finding: internal/dataentry/sections.go:339 says 'populates AddInfo and LinkInfo using full view config' — but the function now has exactly one caller (the side-panel handler), and that caller hand-builds a synthetic ViewConfig from form.SidePanel. Comment should reflect side-panel-only contract so future readers understand why entity-detail view does not call it.
severity: significant
resolution: Rewrote the doc comment on resolveSectionButtonsWithTraverse in internal/dataentry/sections.go to make the side-panel-only contract explicit, including a sentence explaining why entity-detail view does not call it and noting that the viewConfig parameter is a synthetic config built from form.SidePanel.
status: addressed
---
