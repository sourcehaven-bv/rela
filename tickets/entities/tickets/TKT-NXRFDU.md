---
id: TKT-NXRFDU
type: ticket
title: Writes require read access to the existing row
kind: enhancement
priority: high
effort: l
tags: security
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

## Description

The entity manager authorizes a write on the type, id and face, but it does not
check that the caller can read the existing row. `DeleteEntityFace` and
`RenameEntity` do; update, delete, soft-delete, the copy target and relation
writes do not. Most served entry points resolve the row through the read gate
first, but CalDAV PUT and DELETE do not: they fall back to the raw id, so a
caller with the write grant can update or delete a row they cannot read.

Move the rule into the manager, so every write path gets it:

- Update, delete, soft-delete, rename and the copy target require that the caller can read the existing row (face). A refusal is answered as "not found", before the write authorization, so a 403 cannot confirm that the row exists.
- Relation writes require that the caller can read both endpoints. Keep the BUG-K6FEVB order where it matters: no lookup result may leak before authorization.
- Handles without a face gate stay exempt: NopACL, ReadOnlyACL, bypass_acl, cascade writes. Raw-store writers (migrations, importer, normalize) are out of scope.
- An update that makes the row unreadable to the writer afterwards is allowed.
- `PatchEntity` and `DeleteEntity` docs (manager.go and internal/entitymanager/CLAUDE.md) currently accept this existence disclosure; update them.

Coordinated with the arch/acl-store-adapters refactor: the rule lives in the
manager so the handler-side early authorization picks it up.

## Acceptance

- Per write path (HTTP, remote MCP, Lua, webhook, CalDAV, copy), update and delete on a row the caller cannot read give the same result as on a row that does not exist.
- Relation create, update and delete with an unreadable endpoint are refused as not found.
- Migrations, cascades and bypass handles keep working.
- docs/acl-security.md (from its GUIDE entity) states the rule.
