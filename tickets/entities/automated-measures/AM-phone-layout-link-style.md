---
id: AM-phone-layout-link-style
type: automated-measure
title: Phone layout, link style and plural labels stay right
description: E2e tests pin that a list keeps its search box on a phone, leaves no empty band and casts no drawer shadow. Vitest tests pin the link style and accessible name, the dashboard phone labels and short values, and the plural label and empty_message in list text. Go tests pin label_plural on the wire and empty_message in the list config.
kind: test
location: e2e/tests/mobile-layout.spec.ts, frontend UrlWidget.test.ts, DashboardView.test.ts, dashboardCards.test.ts, EntityList.empty.test.ts, SortMenu.test.ts, screenEdits.test.ts, internal/dataentry/entity_type_label_test.go, internal/dataentryconfig/config_test.go
status: active
---

Fails when a list loses its search box on a phone, when the closed drawer casts
a shadow, when a link with a link text loses the theme style or reads its URL
as its name, or when list text builds a plural from the type name.
