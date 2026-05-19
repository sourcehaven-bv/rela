---
id: FEAT-AESD4
type: feature
title: Authorization for data-entry and MCP
summary: Principal-aware ACL with declarative roles, graph-based local roles, groups, and write-time Lua escape hatch.
description: 'Four-layer authorization model on top of Principal{User, Tool}: users → groups → roles → local roles. Declarative acl.yaml drives static role definitions and global assignments; role-conferring graph relations carry per-entity bindings; member-of relations resolve groups transitively (cap 5). Semantics are union of grants plus optional explicit-deny rules. Tamper resistance uses Plone''s delegate-X permission pattern (granting role X requires holding delegate-X), so no separate ACL store or hardcoded admin. Reads filter at entity-level (visibility) and property-level (redaction); list responses carry filtered_count so callers see what was hidden. Every deny names the rule that fired. Lua only at write-time via the existing automation engine — never on the read path. MCP transport intersects user capabilities with agent scope and defaults agents to read-only. Staged delivery: v0 declarative write-side, v1 read filtering + groups + MCP intersection, v2 containment inheritance + per-property writes + explicit-deny, v3 documented automation patterns (no new Lua surface). Design doc: .ignored/acl-design.md. Research basis: cross-system survey of Plone, Casbin, OpenFGA/Zanzibar, Cerbos, Oso, Postgres RLS, AWS IAM, Django Guardian, Apache Ranger, Neo4j.'
priority: medium
status: proposed
---
