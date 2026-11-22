---
id: AM-api-client-rejects-html
type: automated-measure
title: API client rejects an HTML answer
description: A unit test sends HTML, JSON, empty, action text and text-type answers through the real API client.
kind: test
location: frontend/src/api/htmlResponse.test.ts
status: active
---

Fails when an HTML page reaches a caller as data, or when an action's own text
body is rejected.
