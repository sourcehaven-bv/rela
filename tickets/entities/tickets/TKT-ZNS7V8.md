---
id: TKT-ZNS7V8
type: ticket
title: Gate relation grants and attachment writes on MCP
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

MCP on rela-server honors `fields:` grants since TKT-0XL8MF but not `relations:`
grants (create, remove, meta fields) or attachment writes. The data-entry API
checks relations through `validateRelationsModernAffordances`; attachments have
only a row check on both surfaces. Found in the TKT-0XL8MF review.
