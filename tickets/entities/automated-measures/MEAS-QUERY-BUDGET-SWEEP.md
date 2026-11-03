---
id: MEAS-QUERY-BUDGET-SWEEP
type: automated-measure
title: 'Budget sweep: every SPA request shape has a flat query count under a realistic ACL'
description: 'Every storetest.Counting budget test also runs under a realistic ACL (role relations, one declared against the edge direction, inheritance through containment) and fails when reads differ at 10 and 50 rows. New budget tests are covered without opting in. Verified: with the local-role prime disabled, 8 budget tests fail at 1,000 to 2,200 reads.'
kind: test
location: internal/dataentry/querybudget_test.go (readsForOn + realisticBudgetPolicy)
status: active
---
