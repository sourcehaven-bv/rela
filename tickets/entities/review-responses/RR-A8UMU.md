---
id: RR-A8UMU
type: review-response
title: Importer bypasses the guard
finding: internal/importer/importer.go calls store.CreateEntity directly, bypassing createEntityCore and the new guard. A misconfigured third-party importer could persist entities with IDs the normal write path would reject.
severity: minor
reason: 'Intentionally out of scope for this ticket. The bulk-import path explicitly bypasses the entitymanager (per CLAUDE.md: ''Importers, bulk sync, and formatters bypass automations and talk to the store directly''). Importing pre-existing data must preserve existing IDs regardless of id_type, otherwise round-trip breaks. A separate ticket could evaluate whether to add a post-import verification step.'
status: deferred
---
