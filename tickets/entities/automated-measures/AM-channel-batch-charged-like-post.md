---
id: AM-channel-batch-charged-like-post
type: automated-measure
title: 'Regression test: an invalid channel batch spends rate-limit tokens'
description: 'Asserts that PostBatch charges the rate limit at the same point as a single post: a batch refused before that point is free, a batch with an invalid message spends its tokens and gets a 429 once they are gone, and an over-rate batch is a 400. Catches a batch path that validates before it charges (BUG-IJBPM56).'
kind: test
location: internal/realtime/batch_test.go
status: active
---
