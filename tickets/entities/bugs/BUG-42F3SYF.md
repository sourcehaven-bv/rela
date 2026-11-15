---
id: BUG-42F3SYF
type: bug
title: List filter pushdown drops a list value stored under a scalar declaration
description: A list filter[prop]=v is pushed as a scalar PropEqual (listpushdown.go equalityProps), so a row whose string-declared property holds a list such as [open] does not match. The Go filter path (internal/filter via propmatch) reads a list as membership and keeps the row. Found by the TKT-I39Y7N4 differential.
priority: low
status: backlog
---

Seed a ticket whose `status` (declared `string`) is the list `["open"]`, then
GET `/api/v1/tickets?filter[status]=open&sort=title` on a request the store
pages. The ticket is missing; on the Go path it is present. Decide which reading
is right and make both paths agree.
