---
id: TKT-9Q2TX
type: ticket
title: Clone live entity before mutation in handlers
kind: refactor
priority: low
effort: s
status: backlog
---

## Problem\n\nMutation handlers in internal/dataentry/api_v1.go and internal/dataentry/handlers_api.go currently do:\n\n`go\nentity, _ := a.Graph().GetNode(id)\noldEntity := entity.Clone()\nentity.Properties[k] = v        // mutates the LIVE graph node's map\nentity.Content = newContent     // mutates the LIVE field\nws.UpdateEntity(entity, oldEntity)\n`\n\nThe middle two lines mutate the **live** graph node — the same `*model.Entity` that lock-free readers (Search, analyze, view rendering) can be iterating concurrently. writeMu serializes writers against each other but not against readers.\n\nThis is a **pre-existing race** that long predates the App.mu refactor (the entity property map has never been synchronized). The refactor surfaces it more sharply because there's no longer even a token RLock between readers and writers.\n\nFix: clone the entity twice in every mutation handler — once for `oldEntity` (used for the diff), once for the working copy that gets mutated and passed to UpdateEntity. The workspace's UpdateEntity then atomically swaps the live node via Graph.mu.\n\n## Scope\n\n- internal/dataentry/api_v1.go — handleV1UpdateEntity, handleV1SetProperty\n- internal/dataentry/handlers_api.go — handleAPIUpdateEntity\n- internal/dataentry/handlers.go — handleToggleCheckbox is already fixed in TKT-PYN1c per RR-AF6SJ\n- Audit cmd/rela CLI commands and internal/mcp tools for the same pattern\n\n## Acceptance\n\n1. No mutation handler mutates the live `entity` returned by `Graph().GetNode(id)`. All in-place writes happen on a clone.\n2. A new race-detector test exercises a concurrent reader iterating entity properties while a writer updates the same entity. Passes under -race.\n3. The pre-existing race documented in TestConcurrentReloadStateSnapshot's scope notes is no longer a known limitation; the test can be extended to iterate entity properties.\n\n## Origin\n\nDeferred from TKT-PYN1c (PR #346). See RR-1CO05 for the cranky-review finding.
