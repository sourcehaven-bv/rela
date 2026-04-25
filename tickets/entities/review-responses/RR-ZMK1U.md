---
id: RR-ZMK1U
type: review-response
title: Hardcoded literals in e2e assertions
finding: document-edit-button.spec.ts hardcodes 'feature_summary', 'FEAT-001', 'feature' in 4 places (return_to assertion, two waitForURL regexes, and PATCH URL match). Renaming the doc or form in the fixture silently desyncs the assertions. Hoist to local consts at the top of the spec, or add to the SEED-style constants in fixtures.ts.
severity: minor
resolution: Hoisted DOC_WITH_EDIT, DOC_NO_EDIT, FEATURE_ID, EDIT_FORM, EDIT_LABEL to consts at the top of document-edit-button.spec.ts. Renaming any fixture identifier now surfaces as a focused failure rather than scattered string drift.
status: addressed
---
