---
id: AM-not-found-and-refused-requests
type: automated-measure
title: Not-found answers and requests after a refusal
description: Go view tests pin the uniform 404 for a missing view entry; vitest pins the not-found state on the entity page, list and form, the client's request guard and the stream stop under a refusal; the e2e spec opens three addresses that name nothing.
kind: test
location: internal/dataentry/viewworld_absent_test.go, frontend/src/api/workspaceAccess.test.ts, frontend/src/composables/useEvents.test.ts, e2e/tests/not-found.spec.ts
status: active
---

Fails when a missing entry answers anything but the uniform 404, a surface
shows the raw error with Retry, or the client sends a request after a session
or workspace refusal.
