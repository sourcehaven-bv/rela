---
id: CON-command-protocol
type: concept
title: "Command Protocol"
summary: "The ::rela:: line protocol for structured communication between commands and the UI"
---

Commands write structured JSON messages to stdout prefixed with `::rela::` to control
the data entry UI. Each line is parsed independently: lines with the prefix are decoded
as protocol messages, lines without it are treated as log output.

Supported message types: `message` (toast notification), `file` (open or reveal a file),
`entity` (entity update notification with link), `group`/`endgroup` (collapsible grouping),
`open` (open URL in browser), and `error` (error toast). Commands also receive context
via stdin JSON and environment variables like `RELA_PROJECT_ROOT`, `RELA_ENTITY_ID`, and
`RELA_CONTEXT`.
