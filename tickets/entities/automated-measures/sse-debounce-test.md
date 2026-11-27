---
id: sse-debounce-test
type: automated-measure
title: 'Test: SSE burst debounces to one frame'
description: A burst of same-type events yields exactly one frame, however slow the feed is.
kind: test
location: internal/dataentry/sse_acl_test.go
status: active
---

Waits for the first frame, then one more flush window, then asserts one frame.
