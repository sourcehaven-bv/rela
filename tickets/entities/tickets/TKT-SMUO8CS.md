---
id: TKT-SMUO8CS
type: ticket
title: 'Push feed and CalDAV where: filters into the store'
kind: chore
priority: low
effort: m
status: backlog
---

## Description

Follow-up of TKT-C5H506O. `feedEntitySource.listType`
(`internal/dataentry/feed_handler.go`) reads every readable row of a feed source
type, and the ICS/JSON feed and CalDAV collections then filter by `where:` in
Go. The CalDAV driver list (`expandPattern`) does the same for the driver type.
Lower `where:` (and a `since` watermark) into the store query, then remove its
`request-path` entry in `internal/archguard/unpagedread_allowlist_test.go`.
