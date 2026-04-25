---
id: RR-M2HUU
type: review-response
title: fixture form 'feature' has no explicit mode
finding: 'e2e/tests/fixtures.ts forms.feature has no mode: declared. The ''saving'' test asserts a PATCH, which presumes edit mode. If forms.feature is later set to mode: create, this test silently breaks. Either pin mode: edit explicitly or document why no-mode is sufficient.'
severity: minor
resolution: 'Added a dedicated `feature_edit` form to the fixture (mode: edit) and pointed documents.feature_summary.edit.form at it. The shared `feature` form is left alone so existing list/kanban tests still exercise the dual-mode path. The doc-edit-button spec now exercises a deliberately edit-only form.'
status: addressed
---
