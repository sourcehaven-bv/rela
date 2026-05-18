---
id: FEAT-audit-log
type: feature
title: "Audit Log"
status: published
summary: "Forensic JSONL log of every entity / relation write across all entry points"
---

Append-only JSONL records under `.rela/audit/YYYY-MM-DD.jsonl` with
daily UTC rotation. Each record carries the operating user
(`$USER`), the entry-point tool (`cli` / `mcp` / `data-entry` /
`scheduler` / `desktop`), and — for engine-initiated writes — the
originating automation or schedule. Forensic in scope, not
authoritative: the store is the source of truth.
