---
id: RR-DEMUG
type: review-response
title: pickInRelationPicker missing visibility guard
finding: e2e/pages/form.page.ts:286-290 (pickInRelationPicker) does not await expect(search).toBeVisible() before fill(). Cards-page sibling at relation-cards.page.ts:95 does. Risk of flake on slow render.
severity: minor
resolution: Added await expect(search).toBeVisible() before fill, plus await expect(option).toBeVisible() before click. Matches the pattern in linkTargetByIdWithSearch.
status: addressed
---
