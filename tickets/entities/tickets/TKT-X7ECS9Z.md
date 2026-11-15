---
id: TKT-X7ECS9Z
type: ticket
title: Push kanban and list export conditions into the store
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

Follow-up of TKT-I39Y7N4, which pushes list `condition:`s on the list API, nav
items, dashboard cards and nav status. A kanban board and a list export still
apply their condition in Go after reading every row of the type
(`export_list.go`, the kanban read path). Route both through
`listNarrowing.pushdownPlan` and `lowerViewCondition`, with the same
visible-on-every-row rule, and pin each with a rows-plus-calls budget test.
