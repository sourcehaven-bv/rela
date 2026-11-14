---
id: TKT-410DFPF
type: ticket
title: Fix _views path in retired-id redirect docs
kind: docs
priority: low
effort: xs
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Description

GitHub issue #2187: the acl-security guide section "Retired ids redirect only to
a readable successor" names the data-entry views route as
`/api/v1/_views/<view>/<id>`. `handleV1Views` parses
`/api/v1/_views/{entityType}/{entityId}` and `viewMiss` builds the redirect from
the type name, so the segment is the entity type.

## Fix

Replace `<view>` with `<type>` in
`docs-project/entities/guides/GUIDE-acl-security.md` and regenerate
`docs/acl-security.md`.
