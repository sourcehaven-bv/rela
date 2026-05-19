---
id: audit-log
type: concept
title: Audit log
summary: Append-only JSONL log of every entity/relation create/update/delete, stamped with Principal{User, Tool} and optional triggered-by attribution.
description: 'Forensic record of every successful write to the rela store. Records are JSONL under .rela/audit/YYYY-MM-DD.jsonl, with daily rotation. Each row carries timestamp, op, subject (entity or relation identity), principal (user + tool), and optional triggered_by (automation:<name>, schedule:<task>, cascade:<reason>). Implemented as a store-write observer attached at the entitymanager boundary. The audit-log is the single forensic source: every write path (CLI, MCP, data-entry, scheduler, desktop) flows through entitymanager.Manager and produces a record. Denied writes (when authorization lands) will also be recorded so the forensic story covers both successful and refused changes.'
package: internal/audit
layer: server
status: stable
---
