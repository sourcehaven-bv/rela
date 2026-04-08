---
id: RR-73AG
type: review-response
title: navigateToEntity type assertion is sloppy
finding: 'EntityList.vue navigateToEntity uses `value as string` for non-array branches. Although null is filtered earlier with continue, the assertion papers over an inference gap and doesn''t handle the case of an array of entirely nulls cleanly (passes empty string[]). Fix: replace assertion with explicit narrowing; skip if filtered array is empty.'
severity: minor
resolution: 'EntityList.vue navigateToEntity replaces the `value as string` cast with explicit narrowing: if value === null skip; if array, filter to non-null strings and only assign when non-empty; else assign the string directly. No more assertion lying to the type checker.'
status: addressed
---
