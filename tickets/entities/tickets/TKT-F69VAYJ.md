---
id: TKT-F69VAYJ
type: ticket
title: Bound the list-context command payload read
kind: chore
priority: low
effort: m
status: backlog
---

## Description

Follow-up of TKT-C5H506O. `commandHandler.visibleListRows`
(`internal/dataentry/commands.go`) reads every readable row of the list type,
with bodies, and then applies the list `filters:` in Go. Push the list filters
into the store query (or cap the payload and say so in it), then remove its
`request-path` entry in `internal/archguard/unpagedread_allowlist_test.go`.
