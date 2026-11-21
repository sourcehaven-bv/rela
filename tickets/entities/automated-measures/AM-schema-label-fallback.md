---
id: AM-schema-label-fallback
type: automated-measure
title: Unlabelled columns and view fields show the schema label
description: Go tests pin the schema label fallback for view fields and for list, section, nested and /_config columns; frontend tests pin dashboard headers, currency digits, the title hold and the git capability gate.
kind: test
location: internal/dataentry/schema_labels_test.go, internal/dataentry/handlers_git_test.go, frontend format.test.ts, useDocumentTitle.test.ts, git.test.ts, DashboardView.test.ts
status: active
---

Fails when a column or field without `label:` shows the raw property name
while the schema labels it, when a whole currency amount loses its decimals,
when the tab shows the bare app name during a navigation, or when the SPA asks
for git status on a server without git.
