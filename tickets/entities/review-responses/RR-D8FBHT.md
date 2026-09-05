---
id: RR-D8FBHT
type: review-response
title: Reload re-ran boot-only derived-schema DDL on postgres
finding: 'assemble calls reconcileDerivedSchemaIfSupported unconditionally, so reload re-ran it. pgstore.Store.Reconcile documents itself BOOT-ONLY and names re-reconciling on reload as a future extension needing its own debounce/lock policy for issuing DDL off a file watcher. Consequences: CREATE/DROP INDEX on every editor autosave, and a metamodel saved mid-edit that still parses but lost a `unique:` would DROP the live index, since reconciliation is desired-state and absent means delete.'
severity: significant
resolution: Added SharedBase.ForReassembly(), which returns a copy marked as re-assembling against an already-open store; assemble skips the store-open-only reconcile when set. The MCP reload path uses it. The version sweep is deliberately still re-run — it carries the metamodel projection and StartVersionSweep replaces the previous sweep rather than stacking one.
status: addressed
---
