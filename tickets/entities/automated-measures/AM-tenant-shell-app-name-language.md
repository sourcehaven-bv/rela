---
id: AM-tenant-shell-app-name-language
type: automated-measure
title: 'Test: the tenant-free shell names the app and its language'
description: Guards against BUG-21N3J4N. The Go tests pin that StaticShell writes app.name and app.language from the shared data-entry.yaml into the title, the html lang and two meta tags, escaped, and that a project without them keeps the built page. The vitest pins that the SPA uses the server language over the browser before its config loads.
kind: test
location: internal/dataentry/staticshell_test.go:TestShellBootFrom, TestBootShell, TestBootShellHandler, TestLoadShellBoot; cmd/rela-server/tenantmode_shell_test.go:TestShellBoot; frontend/src/i18n/bootMeta.test.ts
status: active
---
