---
id: AM-list-pushdown-differential
type: automated-measure
title: Store-paged lists match the Go path per principal
description: assertScopeMatchesGoPath and assertConditionMatchesGoPath serve every query scope and list condition for every principal through the list API on memstore, sqlite and postgres, compare pages and totals with the Go path, and assert which path served the page. TestNavStatus_InStoreMatchesGoPath does the same for nav status markers.
kind: test
location: internal/dataentry/listcondition_pushdown_test.go
status: active
---
