---
id: FEAT-015
status: implemented
summary: Browser-based E2E tests using chromedp to verify form submission works correctly
description: E2E tests using chromedp (headless Chrome) to verify markdown editor and form submission
title: E2E tests for data entry forms
type: feature
---

Added E2E tests for the data entry web application using chromedp (headless Chrome):

- **TestE2E_MarkdownEditorSave**: Verifies that content typed in the EasyMDE markdown editor is correctly saved when the form is submitted
- **TestE2E_FormFieldSubmit**: Verifies that regular form fields are correctly submitted

Run with: `go test -tags=e2e ./internal/dataentry/...`

Requirements:
- Chrome or Chromium installed
- Prototype project at prototypes/data-entry/project
