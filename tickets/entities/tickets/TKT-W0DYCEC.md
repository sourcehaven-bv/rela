---
id: TKT-W0DYCEC
type: ticket
title: Refuse conditions that do not lower into a store query
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

After TKT-8HY05WB a condition that does not lower into a store query
(`queryplan.LowerCondition` declines) still runs on a Go path that reads the
whole type: MCP `list_entities` (`listPage.filterInGo`), list and kanban view
conditions, query scopes, and `desktopnotify`. That path is unbounded and easy
to miss.

Refuse such conditions instead:

- at config load for view conditions, query scopes and next-action
conditions, with a problem that names the leaf that does not lower (date or
decimal ordered comparison, attribute against attribute, arithmetic, over the
tree limits);
- with a clear MCP error for an ad-hoc `list_entities` condition.

Before refusing, audit `tickets/`, `docs-project/` and the examples for
conditions that would stop loading, and give each a lowerable rewrite or a
migration. Dates depend on TKT-4Z2BIRG; land that first or keep dates on the Go
path until it does.

Fields hidden on some rows stay on the Go path until the field-scope rewrite
ticket lands.

## Acceptance

- A config with a non-lowerable condition fails to load with a problem naming
the leaf.
- MCP `list_entities` with such a condition returns an error, not a scan.
- No project in the repo fails to load.
