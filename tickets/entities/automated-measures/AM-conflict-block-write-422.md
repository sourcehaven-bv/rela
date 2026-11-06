---
id: AM-conflict-block-write-422
type: automated-measure
title: A conflict-block refusal answers 422 conflict_block
description: Every data-entry write the file store refuses for a git conflict block in content answers 422 conflict_block without echoing the content; no 500 path answers it.
kind: test
location: internal/dataentry/conflict_block_write_test.go
status: active
---
