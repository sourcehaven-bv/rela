---
id: TKT-A2SOOZZ
type: ticket
title: Roll-up avg function
kind: enhancement
status: backlog
---

## Description

Split from TKT-LTSNHD. Add `avg` to roll-ups over an `integer` or `decimal`
property, with a `decimal` result. Decide the rounding and the result of an
empty set (no value). Implement it in the generic fold and in the pgstore and
sqlitestore SQL, and pin it in storetest.
