---
id: RR-3PJJQ19
type: review-response
title: Untested paths and weak assertions
finding: Edit mode, dirty re-baseline, re-init paths and the payload were untested; the ne check compared two results; case names claimed the pushdown path.
severity: minor
resolution: 'Added baseline, cleared-field, dry-run payload and no-request tests; ne has an explicit want; case names fixed. Edit mode is not tested: both paths return early on isEdit, and re-init paths reuse the initializeDefaults loop that the cached test covers.'
status: addressed
---
