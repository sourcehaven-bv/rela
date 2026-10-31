---
id: TKT-NWZCWLC
type: ticket
title: 'Push a list condition: that lowers exactly into the store (list page and _nav_status)'
kind: enhancement
priority: medium
effort: m
status: ready
---

## Description

A list `condition:` always keeps a request on the Go list path
(`listNarrowing.pushdownPlan` declines when `n.cond != nil`), so the list page
and `/api/v1/_nav_status` read every header of the type and filter in Go. A
query scope already joins the pushdown when `queryplan.LowerScope` lowers it
exactly (TKT-XKCNCL). Do the same for a condition that lowers exactly, for
example atlas's `related(entity, 'heeft_verantwoordelijke', { id =
current_user.id })`.

Exactness matters more than for a scope: the condition is evaluated against
REDACTED rows, so an equality on a property the principal cannot see on every
row must decline (use the gated-query rule of BUG-WJSYQT).

Measured in TKT-YLG695: after priming, `_nav_status` costs ~10 ms on atlas's
size (155 tasks) and 50–100 ms at 20x (3,100 tasks), most of it the whole-type
read.
