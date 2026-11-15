---
id: TKT-C5H506O
type: ticket
title: Give the whole-set scopedHeaders callers their own read function
kind: chore
priority: medium
effort: m
status: backlog
---

## Description

Follow-up of TKT-D3RAB0E (and TKT-F2EJQY8). `scopedHeaders`
(`internal/dataentry/scopedread.go`) keeps a `requestPath` entry in
`internal/archguard/unpagedread_allowlist_test.go` because export, the calendar
feed, the gantt node load and the list-context command payload read the whole
set through it.

Give those callers their own read function with a `batchJob` or `bounded`
allowlist entry and a reason, page the remaining request-path callers, and then
delete `TestUnpagedReadAllowlist_ScopedHeadersIsDebt`. TKT-F2EJQY8 changes the
command payload; start after it merges.
