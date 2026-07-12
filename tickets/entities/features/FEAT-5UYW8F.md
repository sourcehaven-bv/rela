---
id: FEAT-5UYW8F
type: feature
title: pgstore content versioning (time-machine history + diff)
summary: Automatic per-write version history of entity content in the pgstore backend, with view/diff of prior versions and full Principal attribution — the pgstore analogue of git-backed fsstore versioning.
description: In a pgstore-backed deployment, entity writes are in-place SQL UPDATEs, so prior content is lost and the audit log only records that a change occurred, not the pre-change content. This feature adds automatic, per-write version history for entity content in pgstore — the analogue of the free versioning fsstore gets from git. Each version records the full content/properties as of that write, stamped with the Principal that made the edit, the timestamp, and the op/triggered-by attribution already threaded through the entitymanager/audit boundary. Users can list an entity's version history, view any past version, and diff two versions, surfaced through the CLI, the data-entry UI, and MCP. Leans on the monotonic seq marker pgstore already maintains. Distinct from snapshot-versioned ACL (IDEA-CQMKMD), which is about point-in-time read verdicts rather than content history.
priority: medium
status: proposed
---
