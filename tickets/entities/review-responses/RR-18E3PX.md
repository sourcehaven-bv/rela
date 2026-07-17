---
id: RR-18E3PX
type: review-response
title: graph ID-fallback label guard was untested
finding: The graph.go `title == e.ID` guard (prevents a redundant 'ID\nID' label when DisplayTitle falls back to the ID) had no test — every graph_test fixture had a distinct title, so the branch never executed.
severity: significant
resolution: Strengthened the existing TestGenerateDOT_EntityWithoutTitle (which already sets up a titleless entity via testutil.Entity) to assert the label is the bare ID AND does not contain the duplicated 'CMP-001\nCMP-001' form. (An initial attempt to add a titleless REQ-003 via testutil.EntityFor failed because that helper auto-fills required properties with fake data — used testutil.Entity's existing no-fill fixture instead.)
status: addressed
---
