---
id: RR-MCP1CO
type: review-response
title: Everyone-block warning parses face keys differently from the resolver
finding: fieldBlocksOf split at the first @ and kept keys like document@concept@x that the resolver refuses.
severity: minor
resolution: fieldBlocksOf now skips a key whose face does not parse or holds a second separator.
status: addressed
---
