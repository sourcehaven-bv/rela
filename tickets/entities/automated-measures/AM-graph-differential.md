---
id: AM-graph-differential
type: automated-measure
title: SQL graph queries match the naive reference on random queries
description: storetest.RunGraphDifferential runs 400 seeded random GraphQuery shapes against pgstore and sqlitestore and compares rows, headers, counts and MatchingIDs with graphquerynaive on the same data. Awkward property names and mixed value types are part of the seed.
kind: test
location: internal/store/storetest/graphdiff.go
status: active
---
