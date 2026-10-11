---
id: TKT-F7EROJ
type: ticket
title: Access log records the route shape, not ids or file names
kind: enhancement
priority: medium
effort: s
started: "2026-10-09"
completed: "2026-10-09"
status: done
description: '--access-log logged the decoded request path, which carries entity ids and attachment file names that can be personal data (GitHub #1782). Log the route shape instead.'
---

## Description

GitHub #1782 (finding 1 from the security review on #1773). `--access-log` logs
`r.URL.Path` in full. Entity ids and attachment file names such as
`.../_attachments/file/ziekmelding.pdf` reach journald and central logging. The
log is for timing, which does not need the resource identity. RR-090JDW and
RR-YD6YFX accepted this as residual risk without a follow-up.

## Approach

`routeShape` keeps a path segment only if it is a fixed route word or a schema
name (entity type, plural, relation); every other segment becomes `*`. It is an
allowlist, so a new route degrades to `*` rather than leaking.

## Acceptance criteria

1. `/api/v1/tickets/TKT-1/_attachments/file/x.pdf` logs as
`/api/v1/tickets/*/_attachments/file/*`.
2. Ids, file names and config names never appear in the record.
3. Schema names follow the current schema.
4. Truncation still caps the record.
