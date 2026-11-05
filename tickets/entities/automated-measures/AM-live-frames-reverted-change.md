---
id: AM-live-frames-reverted-change
type: automated-measure
title: Live view frames a change the client saw that was reverted before the next round
description: A dataentry live test claims and releases a lease between two live rounds after the client refetched the claimed state; the subscription must still get a live:changed frame.
kind: test
location: internal/dataentry/live_served_test.go
status: active
---
