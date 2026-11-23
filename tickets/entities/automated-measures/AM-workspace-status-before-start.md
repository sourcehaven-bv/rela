---
id: AM-workspace-status-before-start
type: automated-measure
title: Workspace status read before start, access page titles and read-only text
description: vitest pins that the tenant status records a suspended, archived or refused workspace, keeps the first refusal and gives up after its timeout; that each access page names the tab; and that the read-only banner shows account.read_only_text from the shell. Go tests pin the config key and the shell meta.
kind: test
location: frontend/src/api/workspaceReadOnly.test.ts, frontend/src/components/common/WorkspaceAccessScreen.test.ts, frontend/src/components/common/ReadOnlyBanner.test.ts, internal/dataentry/staticshell_test.go
status: active
---

Fails when the status read stops recording a blocked workspace, an access page
leaves the tab to the app name, or the read-only text no longer reaches the
banner.
