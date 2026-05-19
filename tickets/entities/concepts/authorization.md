---
id: authorization
type: concept
title: Authorization
summary: Principal-aware access control for writes and reads on the data-entry server and MCP transport.
description: 'Authorization model that gates writes (HTTP 403 + reason) and filters reads (entity-level visibility + property-level redaction) based on the request Principal{User, Tool}. Four conceptual layers: users (resolved against a configured user entity type), groups (membership via member-of relations, transitive), roles (named capability bundles in acl.yaml), local roles (role-conferring graph relations from principal/group to entity, optionally inherited along containment relations). Semantics: union + explicit-deny (most-permissive grants win; explicit-deny is a separate named mechanism). Tamper resistance via Plone-style delegate-X permissions (granting role X requires holding delegate-X permission). Lua escape hatch only at write-time via the existing automation engine — never on the read path. MCP transport intersects user capabilities with agent scope and defaults agents to read-only.'
package: internal/acl
layer: server
status: draft
---
