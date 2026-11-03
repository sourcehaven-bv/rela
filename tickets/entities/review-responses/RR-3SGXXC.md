---
id: RR-3SGXXC
type: review-response
title: BusyColumn story does not check the compressed empty slot
finding: AC 12 says the empty cell keeps its slot in the compressed layout. RlTable.stories.ts:744 play only checks the accessible name and "+1". Add an assertion that the idle row in the compress table still renders a busy cell element, so a library change that collapses empty primary cells fails the story run.
severity: nit
resolution: ADDRESSED BusyColumn play now finds the idle row in the compress table and asserts it still has an empty primary cell whose computed display is not none.
status: addressed
---
