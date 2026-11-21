---
id: AM-workspace-read-only-create-form
type: automated-measure
title: Read-only workspace keeps a usable create form
description: Vitest tests serve a refused dry-run through the real API client and pin that the create form keeps its fields, shows the read-only notice and posts nothing. A unit test pins when the read-only flag is set and cleared. TestReadOnly pins the workspace wording.
kind: test
location: frontend/src/components/forms/DynamicForm.readonly.test.ts, frontend/src/api/workspaceReadOnly.test.ts, internal/tenanthost/tenantapi_test.go
status: active
---

Fails when a failed dry-run hides the create form's fields, when a read-only
workspace still offers Create, or when the refusal names the tenant.
