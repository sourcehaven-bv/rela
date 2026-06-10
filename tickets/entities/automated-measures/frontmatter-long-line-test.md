---
id: frontmatter-long-line-test
type: automated-measure
title: 'Test: frontmatter split handles lines over 64KB (round-trip)'
description: 'Regression for BUG-LSBFD1: the leaf-package tests split >64KB lines in body and frontmatter without error; the fsstore test writes an entity with a 256KB body line and reads it back (same instance + after reopen). Fails if the split reverts to a size-capped bufio.Scanner (bufio.Scanner: token too long).'
kind: test
location: internal/frontmatter/frontmatter_test.go (TestSplit_LongLineExceeds64KB, TestSplit_LongLineInFrontmatter, FuzzSplit) + internal/store/fsstore/longline_test.go (TestLongLine_WriteThenReadBack)
status: active
---
