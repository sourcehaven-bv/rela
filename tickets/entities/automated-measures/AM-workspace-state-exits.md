---
id: AM-workspace-state-exits
type: automated-measure
title: Read-only hides record writes; closed workspaces offer a way out
description: Vitest tests pin that every write verb and the record's duplicate, delete and convert are hidden in a read-only workspace, the logo, account and sign-out links and contact text of the workspace pages, the shell banner slot and the provisioning POST. Go tests pin the shell values, the config keys, POST _me provisioning and the dark mail body.
kind: test
location: frontend/src/api/workspaceReadOnly.test.ts, frontend/src/components/entity/EntityDetail.header.test.ts, frontend/src/components/common/WorkspaceAccessScreen.test.ts, frontend/src/components/common/ReadOnlyBanner.placement.test.ts, frontend/src/api/me.test.ts, internal/dataentry/staticshell_test.go, internal/dataentry/provision_e2e_test.go, internal/dataentryconfig/account_test.go, internal/mailrender/compat_test.go
status: active
---

Fails when a read-only workspace offers a record write, when a suspended or
archived workspace page loses its links, when the banner moves back into the
content pane, when opening the app stops provisioning a member, or when a dark
mail keeps a light body.
