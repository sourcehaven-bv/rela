---
id: FEAT-831A
type: feature
title: Audit log of entity write operations
summary: Append-only structured log of every entity/relation create/update/delete, written by EntityManager.
description: 'Provides forensic visibility into what changed, when, and (best-effort) by whom. Records are written as JSONL under .rela/audit/ with daily rotation. Each record carries timestamp, op, entity_type, entity_id, actor (best-effort string today; structured principal later), triggered_by (e.g. automation:<name>, scheduler:<name>), and a short human-readable summary. Useful single-user (debugging the scheduler, automations, MCP/LLM-driven changes) and a foundation for later multi-user features: when principal lands, actor becomes structured; when write-policy lands, denied attempts get logged with outcome=denied. The EntityManager dispatch path established by this feature is the same hook subsequent phases plug into.'
priority: medium
status: proposed
---
