---
id: TKT-A6A97WV
type: ticket
title: Budget tests count rows read, not only store calls
kind: enhancement
priority: high
effort: s
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

## Description

`storetest.Counting` counts store calls, so one call that scans every row passes
a budget test. Count rows returned too (and rows from `store.QueryStats` on
postgres), add a helper that asserts rows-read is the same with 100 and 1\,000
entities in the store, and convert the existing budget tests to it.

## Done when

- The helper exists and every existing budget test asserts rows as well as calls.
- A test proves a single full-scan call fails the helper.
