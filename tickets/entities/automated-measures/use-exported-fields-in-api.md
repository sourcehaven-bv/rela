---
id: use-exported-fields-in-api
kind: test
location: internal/mcp/tools.go
status: active
title: Use exported fields in API responses
description: Use exported struct fields in JSON API responses for proper marshaling
type: automated-measure
---

Always use exported (capitalized) struct fields when building JSON API responses to ensure proper marshaling.
