---
id: bleve-id-prefix-ranking-test
type: automated-measure
title: Bleve id-prefix ranking regression test
description: TestIndex_SearchByIDPrefix in internal/search/bleveindex/bleveindex_test.go seeds PRS-ACT-* (title-only) and VAD-ACT-* (id-prefix) entities, searches 'VAD-ACT-', and asserts the id-prefix matches occupy the top ranks. Fails CI if the bleve id-field exact/prefix boosting regresses. Paired with a backspace-across-prefix recovery test in useBacktickAutocomplete.test.ts on the frontend layer.
kind: test
location: internal/search/bleveindex/bleveindex_test.go
status: active
---

## What it guards

The entity-reference picker relevance fix (BUG-O09QUC): typing an ID prefix must
rank ID-prefix matches above incidental title matches.

## Test

`TestIndex_SearchByIDPrefix` (`internal/search/bleveindex/bleveindex_test.go`)
reproduces the reported `VAD-ACT-` scenario and asserts the three `VAD-ACT-*`
entities occupy the top three ranks ahead of `PRS-ACT-*` title-only matches.
