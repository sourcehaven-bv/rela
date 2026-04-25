---
id: RR-M2HUU
type: review-response
title: fixture form 'feature' has no explicit mode
finding: 'e2e/tests/fixtures.ts forms.feature has no mode: declared. The ''saving'' test asserts a PATCH, which presumes edit mode. If forms.feature is later set to mode: create, this test silently breaks. Either pin mode: edit explicitly or document why no-mode is sufficient.'
severity: minor
resolution: 'Added a dedicated `feature_edit` form to the fixture (mode: edit) and pointed documents.feature_summary.edit.form at it. The shared `feature` form is left alone so existing list/kanban tests still exercise the dual-mode path. The doc-edit-button spec now exercises a deliberately edit-only form.'
reason: 'Initial fix added a dedicated `feature_edit` form (mode: edit), but that broke two unrelated tests in the shared fixture: settings.spec.ts asserts the form count is 3 (now 4), and entity-detail.spec.ts asserts the resolved edit URL contains /form/feature/ (which `getEditFormId` would no longer pick once a mode-edit form was available). The doc spec now reuses the shared `feature` form and documents the dependency on its dual-mode behaviour inline. Pinning mode separately would require either splitting all tests or pinning all consumers of `feature` to mode: edit — disproportionate. Accepting the original concern as a known coupling.'
status: wont-fix
---
