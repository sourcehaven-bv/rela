---
id: AM-workspace-access-states
type: automated-measure
title: Each workspace and session state has its own code and screen
description: Go tests pin the refusal code per tenant state in the host and the status route. Vitest tests pin the refusal each response stands for, the screen and its actions, the session probe, the read-only banner and the hidden create buttons.
kind: test
location: internal/tenanthost/host_test.go, internal/tenanthost/provision_test.go, internal/tenant/cluster_test.go, frontend/src/api/workspaceAccess.test.ts, frontend/src/components/common/WorkspaceAccessScreen.test.ts, frontend/src/components/common/ReadOnlyBanner.test.ts
status: active
---

Fails when a suspended or archived workspace is answered as unknown, when an
ended session shows a network error, or when a read-only workspace offers
Create without saying why.
