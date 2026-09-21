---
id: RR-0GQVYU
type: review-response
title: selectType left the highlight on an arbitrary entity from the pre-scope result set
finding: '`selectType` set `selectedType`, blanked `typeItems` and scheduled a search, but never reset the highlight. One frame after clicking the SECOND type row the highlight sat on the second ENTITY of the unscoped result set, which may not even belong to the type just picked; Enter inserted it. Reachable by mouse as well as keyboard, since `onMenuPick` calls `setHighlight(index)` before `commitChoice`, so clicking the third type suggestion left the highlight on the third entity.'
severity: critical
resolution: '`selectType`/`clearType` now route through a shared `applyScopeChange()` that clears `state.items` and resets `state.highlight` before scheduling the scoped search. Combined with the identity-based highlight, there is no stale row left to point at. Pinned by ''does not leave the highlight on a stale entity after a scope is picked''; mutation-verified (removing the two resets fails exactly the three scope-change tests).'
status: addressed
---
