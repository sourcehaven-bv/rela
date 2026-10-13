---
id: BUG-VUNO11
type: bug
title: MCP update_entity ignores field-level write grants
description: MCP update_entity writes fields the principal has no field-level write grant for; REST answers 403.
priority: high
status: backlog
---

## Description

MCP `update_entity` does not check field-level write grants. In the TKT-0M4H0S
golden corpus, the editor writes the `note` field over MCP and the write
succeeds (`mcp-update-denied-field.editor`). The same write over REST answers
403.

## Fix

Route MCP writes through the same field-write authorization as REST
(`affordances` field-write check) before `PatchEntity`. Pin with the corpus case
and a unit test.
