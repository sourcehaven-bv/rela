---
id: RR-FRLI9W
type: review-response
title: External-create restore may attach old channels to a new entity
finding: The reconciler ledger lives in one process's memory and is cleared only by that process's manager creates. With rela-server and a CLI or MCP process on one project (or several postgres nodes), a new entity created through another process's manager gets the old entity's channels restored when this process sees the Created event.
severity: significant
status: open
---
